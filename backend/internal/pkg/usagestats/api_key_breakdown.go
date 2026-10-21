package usagestats

import "time"

type APIKeyBreakdownQuery struct {
	UserID, APIKeyID, GroupID int64
	StartTime, EndTime        time.Time
	APIKeyUsageTrendOptions
}

type APIKeyModelUsage struct {
	APIKeyUsageTotals
	Model string `json:"model"`
}

type APIKeyGroupUsage struct {
	APIKeyUsageTotals
	GroupID       *int64             `json:"group_id"`
	GroupName     string             `json:"group_name"`
	GroupDeleted  bool               `json:"group_deleted"`
	GroupPlatform string             `json:"group_platform"`
	Models        []APIKeyModelUsage `json:"models"`
}

type APIKeyUsageBreakdown struct {
	APIKeyID        int64               `json:"api_key_id"`
	Name            string              `json:"name"`
	Deleted         bool                `json:"deleted"`
	StartTime       time.Time           `json:"start_time"`
	EndTime         time.Time           `json:"end_time"`
	Summary         APIKeyUsageTotals   `json:"summary"`
	Groups          []APIKeyGroupUsage  `json:"groups"`
	PreviousSummary *APIKeyUsageCompare `json:"previous_summary"`
	Series          []APIKeyUsageBucket `json:"series"`
}
