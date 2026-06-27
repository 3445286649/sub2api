package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelRadarRepository struct {
	db *sql.DB
}

func NewModelRadarRepository(db *sql.DB) service.ModelRadarRepository {
	return &modelRadarRepository{db: db}
}

func (r *modelRadarRepository) CreateRun(ctx context.Context, run *service.ModelRadarRun) error {
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	err := r.db.QueryRowContext(ctx, `
INSERT INTO model_radar_runs
  (run_date, trigger_type, status, published, started_at, finished_at, total_combinations, success_combinations, error_message, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, created_at, updated_at
`, run.RunDate, run.TriggerType, run.Status, run.Published, run.StartedAt, run.FinishedAt, run.TotalCombinations, run.SuccessCombinations, run.ErrorMessage, run.CreatedAt, run.UpdatedAt).
		Scan(&run.ID, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create model radar run: %w", err)
	}
	return nil
}

func (r *modelRadarRepository) GetScheduledRunByDate(ctx context.Context, runDate time.Time) (*service.ModelRadarRun, error) {
	return r.scanRun(ctx, `
SELECT id, run_date, trigger_type, status, published, started_at, finished_at, total_combinations, success_combinations, error_message, created_at, updated_at
FROM model_radar_runs
WHERE trigger_type = 'scheduled' AND run_date = $1
ORDER BY id DESC
LIMIT 1
`, runDate)
}

func (r *modelRadarRepository) UpdateRun(ctx context.Context, run *service.ModelRadarRun) error {
	run.UpdatedAt = time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
UPDATE model_radar_runs
SET status=$2, published=$3, started_at=$4, finished_at=$5, total_combinations=$6, success_combinations=$7, error_message=$8, updated_at=$9
WHERE id=$1
`, run.ID, run.Status, run.Published, run.StartedAt, run.FinishedAt, run.TotalCombinations, run.SuccessCombinations, run.ErrorMessage, run.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update model radar run: %w", err)
	}
	return nil
}

func (r *modelRadarRepository) InsertResult(ctx context.Context, result *service.ModelRadarResult, tasks []*service.ModelRadarTaskResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	result.CreatedAt = now
	result.UpdatedAt = now
	err = tx.QueryRowContext(ctx, `
INSERT INTO model_radar_results
  (run_id, model, reasoning_effort, score, pass_count, total_count, avg_latency_ms, error_count, status, rank, error_message, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (run_id, model, reasoning_effort) DO UPDATE SET
  score=EXCLUDED.score,
  pass_count=EXCLUDED.pass_count,
  total_count=EXCLUDED.total_count,
  avg_latency_ms=EXCLUDED.avg_latency_ms,
  error_count=EXCLUDED.error_count,
  status=EXCLUDED.status,
  rank=EXCLUDED.rank,
  error_message=EXCLUDED.error_message,
  updated_at=EXCLUDED.updated_at
RETURNING id, created_at, updated_at
`, result.RunID, result.Model, result.ReasoningEffort, result.Score, result.PassCount, result.TotalCount, result.AvgLatencyMs, result.ErrorCount, result.Status, result.Rank, result.ErrorMessage, result.CreatedAt, result.UpdatedAt).
		Scan(&result.ID, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert model radar result: %w", err)
	}
	for _, task := range tasks {
		task.ResultID = result.ID
		task.CreatedAt = now
		if _, err := tx.ExecContext(ctx, `
INSERT INTO model_radar_task_results
  (result_id, task_id, task_version, passed, expected_answer, actual_answer, latency_ms, error_message, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (result_id, task_id) DO UPDATE SET
  task_version=EXCLUDED.task_version,
  passed=EXCLUDED.passed,
  expected_answer=EXCLUDED.expected_answer,
  actual_answer=EXCLUDED.actual_answer,
  latency_ms=EXCLUDED.latency_ms,
  error_message=EXCLUDED.error_message
`, task.ResultID, task.TaskID, task.TaskVersion, task.Passed, task.ExpectedAnswer, task.ActualAnswer, task.LatencyMs, task.ErrorMessage, task.CreatedAt); err != nil {
			return fmt.Errorf("insert model radar task result: %w", err)
		}
	}
	return tx.Commit()
}

func (r *modelRadarRepository) ListRuns(ctx context.Context, limit int) ([]*service.ModelRadarRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_date, trigger_type, status, published, started_at, finished_at, total_combinations, success_combinations, error_message, created_at, updated_at
FROM model_radar_runs
ORDER BY id DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModelRadarRuns(rows)
}

func (r *modelRadarRepository) GetRun(ctx context.Context, id int64) (*service.ModelRadarRun, error) {
	return r.scanRun(ctx, `
SELECT id, run_date, trigger_type, status, published, started_at, finished_at, total_combinations, success_combinations, error_message, created_at, updated_at
FROM model_radar_runs
WHERE id=$1
`, id)
}

func (r *modelRadarRepository) ListResults(ctx context.Context, runID int64) ([]*service.ModelRadarResult, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, run_id, model, reasoning_effort, score, pass_count, total_count, avg_latency_ms, error_count, status, rank, error_message, created_at, updated_at
FROM model_radar_results
WHERE run_id=$1
ORDER BY rank ASC, score DESC, id ASC
`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModelRadarResults(rows)
}

func (r *modelRadarRepository) ListTaskResults(ctx context.Context, resultIDs []int64) (map[int64][]*service.ModelRadarTaskResult, error) {
	out := map[int64][]*service.ModelRadarTaskResult{}
	if len(resultIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(resultIDs))
	args := make([]any, len(resultIDs))
	for i, id := range resultIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, result_id, task_id, task_version, passed, expected_answer, actual_answer, latency_ms, error_message, created_at
FROM model_radar_task_results
WHERE result_id IN (`+strings.Join(placeholders, ",")+`)
ORDER BY id ASC
`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item := &service.ModelRadarTaskResult{}
		if err := rows.Scan(&item.ID, &item.ResultID, &item.TaskID, &item.TaskVersion, &item.Passed, &item.ExpectedAnswer, &item.ActualAnswer, &item.LatencyMs, &item.ErrorMessage, &item.CreatedAt); err != nil {
			return nil, err
		}
		out[item.ResultID] = append(out[item.ResultID], item)
	}
	return out, rows.Err()
}

func (r *modelRadarRepository) GetLatestPublishedRun(ctx context.Context) (*service.ModelRadarRun, error) {
	return r.scanRun(ctx, `
SELECT id, run_date, trigger_type, status, published, started_at, finished_at, total_combinations, success_combinations, error_message, created_at, updated_at
FROM model_radar_runs
WHERE published = TRUE AND status = 'succeeded'
ORDER BY run_date DESC, id DESC
LIMIT 1
`)
}

func (r *modelRadarRepository) ListPublishedBestResults(ctx context.Context, limit int) ([]*service.ModelRadarResult, error) {
	if limit <= 0 || limit > 30 {
		limit = 7
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT res.id, res.run_id, res.model, res.reasoning_effort, res.score, res.pass_count, res.total_count,
       res.avg_latency_ms, res.error_count, res.status, res.rank, res.error_message, res.created_at, res.updated_at
FROM model_radar_results res
JOIN model_radar_runs run ON run.id = res.run_id
WHERE run.published = TRUE AND run.status = 'succeeded' AND res.rank = 1
ORDER BY run.run_date DESC, run.id DESC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModelRadarResults(rows)
}

func (r *modelRadarRepository) scanRun(ctx context.Context, query string, args ...any) (*service.ModelRadarRun, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	run := &service.ModelRadarRun{}
	err := row.Scan(&run.ID, &run.RunDate, &run.TriggerType, &run.Status, &run.Published, &run.StartedAt, &run.FinishedAt, &run.TotalCombinations, &run.SuccessCombinations, &run.ErrorMessage, &run.CreatedAt, &run.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrModelRadarRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return run, nil
}

func scanModelRadarRuns(rows *sql.Rows) ([]*service.ModelRadarRun, error) {
	items := []*service.ModelRadarRun{}
	for rows.Next() {
		run := &service.ModelRadarRun{}
		if err := rows.Scan(&run.ID, &run.RunDate, &run.TriggerType, &run.Status, &run.Published, &run.StartedAt, &run.FinishedAt, &run.TotalCombinations, &run.SuccessCombinations, &run.ErrorMessage, &run.CreatedAt, &run.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

func scanModelRadarResults(rows *sql.Rows) ([]*service.ModelRadarResult, error) {
	items := []*service.ModelRadarResult{}
	for rows.Next() {
		result := &service.ModelRadarResult{}
		if err := rows.Scan(&result.ID, &result.RunID, &result.Model, &result.ReasoningEffort, &result.Score, &result.PassCount, &result.TotalCount, &result.AvgLatencyMs, &result.ErrorCount, &result.Status, &result.Rank, &result.ErrorMessage, &result.CreatedAt, &result.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, result)
	}
	return items, rows.Err()
}
