package service

import (
	"context"
	"time"
)

const (
	ModelRadarStatusPending   = "pending"
	ModelRadarStatusRunning   = "running"
	ModelRadarStatusSucceeded = "succeeded"
	ModelRadarStatusFailed    = "failed"

	ModelRadarTriggerScheduled = "scheduled"
	ModelRadarTriggerManual    = "manual"

	ModelRadarAPIKeySourceCustom   = "custom"
	ModelRadarAPIKeySourceExisting = "existing"

	modelRadarDefaultRunHour     = 4
	modelRadarDefaultRunMinute   = 30
	modelRadarDefaultTimeoutSec  = 90
	modelRadarDefaultConcurrency = 2
	modelRadarDefaultBudgetCents = 100
)

type ModelRadarConfig struct {
	Enabled             bool                    `json:"enabled"`
	APIBaseURL          string                  `json:"api_base_url"`
	APIKeySource        string                  `json:"api_key_source"`
	APIKeyID            *int64                  `json:"api_key_id,omitempty"`
	APIKeyName          string                  `json:"api_key_name,omitempty"`
	APIKeyGroupName     string                  `json:"api_key_group_name,omitempty"`
	APIKeyStatus        string                  `json:"api_key_status,omitempty"`
	APIKey              string                  `json:"api_key,omitempty"`
	APIKeyConfigured    bool                    `json:"api_key_configured"`
	APIKeyMasked        string                  `json:"api_key_masked"`
	RunHour             int                     `json:"run_hour"`
	RunMinute           int                     `json:"run_minute"`
	TimeoutSeconds      int                     `json:"timeout_seconds"`
	Concurrency         int                     `json:"concurrency"`
	DailyBudgetUSDCents int                     `json:"daily_budget_usd_cents"`
	Matrix              []ModelRadarCombination `json:"matrix"`
}

type ModelRadarCombination struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Enabled         bool   `json:"enabled"`
}

type ModelRadarRun struct {
	ID                  int64      `json:"id"`
	RunDate             time.Time  `json:"run_date"`
	TriggerType         string     `json:"trigger_type"`
	Status              string     `json:"status"`
	Published           bool       `json:"published"`
	StartedAt           *time.Time `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
	TotalCombinations   int        `json:"total_combinations"`
	SuccessCombinations int        `json:"success_combinations"`
	ErrorMessage        string     `json:"error_message"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type ModelRadarResult struct {
	ID              int64     `json:"id"`
	RunID           int64     `json:"run_id"`
	Model           string    `json:"model"`
	ReasoningEffort string    `json:"reasoning_effort"`
	Score           int       `json:"score"`
	PassCount       int       `json:"pass_count"`
	TotalCount      int       `json:"total_count"`
	AvgLatencyMs    *int      `json:"avg_latency_ms"`
	ErrorCount      int       `json:"error_count"`
	Status          string    `json:"status"`
	Rank            int       `json:"rank"`
	ErrorMessage    string    `json:"error_message"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ModelRadarTaskResult struct {
	ID             int64     `json:"id"`
	ResultID       int64     `json:"result_id"`
	TaskID         string    `json:"task_id"`
	TaskVersion    int       `json:"task_version"`
	Passed         bool      `json:"passed"`
	ExpectedAnswer string    `json:"expected_answer"`
	ActualAnswer   string    `json:"actual_answer"`
	LatencyMs      *int      `json:"latency_ms"`
	ErrorMessage   string    `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
}

type ModelRadarCurrent struct {
	Run            *ModelRadarRun      `json:"run"`
	Recommendation *ModelRadarResult   `json:"recommendation"`
	Results        []*ModelRadarResult `json:"results"`
	History        []*ModelRadarResult `json:"history"`
	UpdatedAt      *time.Time          `json:"updated_at"`
}

type ModelRadarRunDetail struct {
	Run         *ModelRadarRun                    `json:"run"`
	Results     []*ModelRadarResult               `json:"results"`
	TaskResults map[int64][]*ModelRadarTaskResult `json:"task_results"`
}

type ModelRadarRunParams struct {
	TriggerType string
	Now         time.Time
}

type ModelRadarConfigUpdateOptions struct {
	AdminUserID int64
}

type ModelRadarRepository interface {
	CreateRun(ctx context.Context, run *ModelRadarRun) error
	GetScheduledRunByDate(ctx context.Context, runDate time.Time) (*ModelRadarRun, error)
	UpdateRun(ctx context.Context, run *ModelRadarRun) error
	InsertResult(ctx context.Context, result *ModelRadarResult, tasks []*ModelRadarTaskResult) error
	ListRuns(ctx context.Context, limit int) ([]*ModelRadarRun, error)
	GetRun(ctx context.Context, id int64) (*ModelRadarRun, error)
	ListResults(ctx context.Context, runID int64) ([]*ModelRadarResult, error)
	ListTaskResults(ctx context.Context, resultIDs []int64) (map[int64][]*ModelRadarTaskResult, error)
	GetLatestPublishedRun(ctx context.Context) (*ModelRadarRun, error)
	ListPublishedBestResults(ctx context.Context, limit int) ([]*ModelRadarResult, error)
}
