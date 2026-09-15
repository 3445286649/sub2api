package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrChannelMonitorFusionQuota = infraerrors.BadRequest("INVALID_MONITOR_FUSION", "V3 data fusion requires a probe monitor")

// Probe buckets contain only public observations, never probe credentials/messages.
type ChannelMonitorProbeBucket struct {
	BucketStart   time.Time `json:"bucket_start"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	LastCheckedAt time.Time `json:"last_checked_at"`
}

type ChannelMonitorFusionRepository interface {
	TryBeginProbe(context.Context, int64, bool) (release func(), acquired bool, err error)
	ProbeBuckets(context.Context, []*ChannelMonitor, ChannelMonitorV2Filter) (map[int64][]ChannelMonitorProbeBucket, error)
}

func ChannelMonitorProbeTargetHash(m *ChannelMonitor) string {
	// Match the actual probe target/options, not the display name or category.
	data, _ := json.Marshal([]any{m.Provider, normalizeEndpoint(m.Endpoint), defaultAPIMode(m.APIMode), m.PrimaryModel, m.ExtraHeaders, defaultBodyMode(m.BodyOverrideMode), m.BodyOverride})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (r ChannelMonitorRuntime) AllowsMonitorProbe(m *ChannelMonitor) bool {
	if r.ReadFailed || !r.Enabled || m == nil || !m.Enabled {
		return false
	}
	if r.Mode == ChannelMonitorModeV1 {
		return true
	}
	return r.Mode == ChannelMonitorModeV3 && m.V3FusionEnabled && defaultCheckMode(m.CheckMode) != MonitorCheckModeQuota
}

func (s *ChannelMonitorService) V3ProbeBuckets(ctx context.Context, cards []*ChannelMonitor, filter ChannelMonitorV2Filter) (map[int64][]ChannelMonitorProbeBucket, error) {
	selected := make([]*ChannelMonitor, 0, len(cards))
	allowed := make(map[int64]bool, len(filter.AllowedGroupIDs))
	for _, id := range filter.AllowedGroupIDs {
		allowed[id] = true
	}
	for _, m := range cards {
		if m.Enabled && m.V3FusionEnabled && m.V3GroupID != nil && (!filter.RestrictGroups || allowed[*m.V3GroupID]) && defaultCheckMode(m.CheckMode) != MonitorCheckModeQuota {
			selected = append(selected, m)
		}
	}
	if len(selected) == 0 {
		return map[int64][]ChannelMonitorProbeBucket{}, nil
	}
	repo, ok := s.repo.(ChannelMonitorFusionRepository)
	if !ok {
		return nil, fmt.Errorf("monitor fusion repository unavailable")
	}
	return repo.ProbeBuckets(ctx, selected, filter)
}

// Scheduled execution uses the same checker as manual V1 checks, but is due-time gated.
func (s *ChannelMonitorService) RunScheduledCheck(ctx context.Context, id int64) ([]*CheckResult, error) {
	return s.runCheck(ctx, id, true)
}
