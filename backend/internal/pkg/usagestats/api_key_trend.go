package usagestats

import "time"

const (
	KeyUsageGranularityHour = "hour"
	KeyUsageGranularityDay  = "day"
)

// APIKeyUsageTrendOptions is shared by the key summary and breakdown queries.
// A nil PrevStartTime disables the comparison; an empty Granularity disables buckets.
type APIKeyUsageTrendOptions struct {
	PrevStartTime, PrevEndTime *time.Time
	Granularity                string
	Location                   *time.Location
}

type APIKeyUsageCompare struct {
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	ActualCost  float64 `json:"actual_cost"`
}

type APIKeyUsageBucket struct {
	APIKeyUsageCompare
	Bucket time.Time `json:"bucket"`
}

// KeyUsageBuckets splits [start, end) into hour buckets anchored at start, or into
// calendar days of loc. Day boundaries come from time.Date so DST days stay 23/25h.
func KeyUsageBuckets(start, end time.Time, granularity string, loc *time.Location) (starts, ends []time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	for s := start; s.Before(end); {
		var next time.Time
		switch granularity {
		case KeyUsageGranularityHour:
			next = s.Add(time.Hour)
		case KeyUsageGranularityDay:
			y, m, d := s.In(loc).Date()
			next = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		default:
			return nil, nil
		}
		if next.After(end) {
			next = end
		}
		starts, ends = append(starts, s), append(ends, next)
		s = next
	}
	return starts, ends
}
