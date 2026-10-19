package usagestats

import "time"

// APIKeySummaryQuery is scoped to the authenticated user by the handler.
type APIKeySummaryQuery struct {
	UserID, APIKeyID, GroupID int64
	StartTime, EndTime        time.Time
	Search, SortBy, SortOrder string
	OnlyUsed                  bool
	Page, PageSize            int
}

type APIKeyUsageTotals struct {
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
	SubscriptionCost    float64 `json:"subscription_cost"`
}

type APIKeyUsageSummaryItem struct {
	APIKeyUsageTotals
	APIKeyID  int64  `json:"api_key_id"`
	Name      string `json:"name"`
	Deleted   bool   `json:"deleted"`
	GroupName string `json:"group_name"`
}

type APIKeyUsageSummary struct {
	Items    []APIKeyUsageSummaryItem `json:"items"`
	Summary  APIKeyUsageTotals        `json:"summary"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}
