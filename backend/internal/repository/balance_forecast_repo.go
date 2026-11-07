package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type balanceForecastRepository struct {
	sql sqlExecutor
}

func NewBalanceForecastRepository(sqlDB *sql.DB) service.BalanceForecastRepository {
	return &balanceForecastRepository{sql: sqlDB}
}

// CloseRecoveredEpisodes 关闭两类轮次：用户余额已高于开轮时（扣费只会让余额下降，
// 所以高于说明充值过），以及用户已被删除、停用或关闭了余额提醒。
func (r *balanceForecastRepository) CloseRecoveredEpisodes(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.sql.ExecContext(ctx, `
		UPDATE balance_forecast_episodes e
		SET status = 'closed', closed_at = $1
		FROM users u
		WHERE e.user_id = u.id
		  AND e.status = 'open'
		  AND (
			u.deleted_at IS NOT NULL
			OR u.status <> 'active'
			OR u.balance_notify_enabled IS NOT TRUE
			OR u.balance > e.balance_at_open
		  )
	`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListCandidates 只统计余额扣费（billing_type = 0，订阅扣费不计），
// 且只返回有已配置通知邮箱的用户，避免对没有收件人的用户做无谓的汇总。
func (r *balanceForecastRepository) ListCandidates(ctx context.Context, since time.Time, minWindowCost float64) ([]service.BalanceForecastCandidate, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT u.id, u.email, u.username, u.balance, s.cost, u.balance_notify_extra_emails
		FROM users u
		JOIN (
			SELECT user_id, SUM(actual_cost) AS cost
			FROM usage_logs
			WHERE created_at >= $1 AND billing_type = 0
			GROUP BY user_id
		) s ON s.user_id = u.id
		WHERE u.deleted_at IS NULL
		  AND u.status = 'active'
		  AND u.balance_notify_enabled IS TRUE
		  AND u.balance > 0
		  AND u.balance_notify_extra_emails NOT IN ('', '[]')
		  AND s.cost >= $2
		ORDER BY u.id
	`, since, minWindowCost)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []service.BalanceForecastCandidate
	for rows.Next() {
		var c service.BalanceForecastCandidate
		if err := rows.Scan(&c.UserID, &c.Email, &c.Username, &c.Balance, &c.WindowCost, &c.ExtraEmails); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// OpenEpisode 幂等：部分唯一索引保证同一用户最多一条 open，冲突时直接读现有轮次。
func (r *balanceForecastRepository) OpenEpisode(ctx context.Context, input service.BalanceForecastEpisodeInput) (*service.BalanceForecastEpisode, error) {
	if _, err := r.sql.ExecContext(ctx, `
		INSERT INTO balance_forecast_episodes (user_id, balance_at_open, daily_burn_at_open, hours_remaining_at_open, opened_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) WHERE status = 'open' DO NOTHING
	`, input.UserID, input.Balance, input.DailyBurn, input.HoursRemaining, input.OpenedAt); err != nil {
		return nil, err
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, notified_at
		FROM balance_forecast_episodes
		WHERE user_id = $1 AND status = 'open'
	`, input.UserID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, rows.Err()
	}
	var (
		episode    service.BalanceForecastEpisode
		notifiedAt sql.NullTime
	)
	if err := rows.Scan(&episode.ID, &notifiedAt); err != nil {
		return nil, err
	}
	if notifiedAt.Valid {
		t := notifiedAt.Time
		episode.NotifiedAt = &t
	}
	return &episode, nil
}

func (r *balanceForecastRepository) MarkEpisodeNotified(ctx context.Context, id int64, at time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
		UPDATE balance_forecast_episodes
		SET notified_at = $2
		WHERE id = $1 AND notified_at IS NULL
	`, id, at)
	return err
}
