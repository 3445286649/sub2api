//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type fusionRepoStub struct {
	quotaModeRepoStub
	claimed  bool
	calls    int
	releases int
	selected []*ChannelMonitor
}

func (r *fusionRepoStub) TryBeginProbe(context.Context, int64, bool) (func(), bool, error) {
	r.calls++
	return func() { r.releases++ }, r.claimed, nil
}
func (r *fusionRepoStub) ProbeBuckets(_ context.Context, cards []*ChannelMonitor, _ ChannelMonitorV2Filter) (map[int64][]ChannelMonitorProbeBucket, error) {
	r.selected = cards
	return map[int64][]ChannelMonitorProbeBucket{}, nil
}

func TestChannelMonitorFusionExecution(t *testing.T) {
	endpoint := setupFakeOpenAI(t, &openAICaptureHandler{})
	repo := &fusionRepoStub{claimed: true, quotaModeRepoStub: quotaModeRepoStub{monitor: &ChannelMonitor{
		ID: 1, Enabled: true, V3FusionEnabled: true, Provider: MonitorProviderOpenAI, APIMode: MonitorAPIModeChatCompletions,
		Endpoint: endpoint, APIKey: "OLD:sk-fixture", PrimaryModel: "primary", ExtraModels: []string{"extra"}, IntervalSeconds: 120,
	}}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV3}})
	results, err := svc.RunScheduledCheck(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "primary", results[0].Model)
	require.Len(t, repo.history, 1)
	require.True(t, repo.history[0].IsProbe)
	require.Len(t, repo.history[0].ProbeTargetHash, 64)
	require.Equal(t, 1, repo.releases)
	repo.monitor.V3FusionEnabled = false
	_, err = svc.RunScheduledCheck(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls, "unselected cards must not claim or probe")
	repo.monitor.V3FusionEnabled = true
	repo.claimed = false
	results, err = svc.RunScheduledCheck(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, results)
	require.Len(t, repo.history, 1, "another replica's claim must prevent probing")
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{ReadFailed: true, Enabled: true, Mode: ChannelMonitorModeV1}})
	_, err = svc.RunScheduledCheck(context.Background(), 1)
	require.ErrorIs(t, err, ErrChannelMonitorDisabled)
	svc.SetRuntimeReader(channelMonitorRuntimeStub{rt: ChannelMonitorRuntime{Enabled: false, Mode: ChannelMonitorModeV3}})
	_, err = svc.RunCheck(context.Background(), 1)
	require.ErrorIs(t, err, ErrChannelMonitorDisabled)
}

func TestChannelMonitorFusionScopeAndConfiguration(t *testing.T) {
	one, two := int64(1), int64(2)
	repo := &fusionRepoStub{}
	svc := NewChannelMonitorService(repo, nil)
	cards := []*ChannelMonitor{
		{ID: 10, Enabled: true, V3FusionEnabled: true, V3GroupID: &one},
		{ID: 20, Enabled: true, V3FusionEnabled: true, V3GroupID: &two},
		{ID: 30, Enabled: true, V3FusionEnabled: false, V3GroupID: &one},
		{ID: 40, Enabled: true, V3FusionEnabled: true, V3GroupID: &one, CheckMode: MonitorCheckModeQuota},
	}
	filter, err := ChannelMonitorWindow("90m", time.Now())
	require.NoError(t, err)
	filter.RestrictGroups = true
	filter.AllowedGroupIDs = []int64{1}
	_, err = svc.V3ProbeBuckets(context.Background(), cards, filter)
	require.NoError(t, err)
	require.Len(t, repo.selected, 1)
	require.Equal(t, int64(10), repo.selected[0].ID)
	require.Error(t, validateCreateParams(ChannelMonitorCreateParams{V3FusionEnabled: true, CheckMode: MonitorCheckModeQuota}))
	m := &ChannelMonitor{V3FusionEnabled: true, Provider: MonitorProviderOpenAI, Endpoint: "https://example.com", APIKey: "fixture", PrimaryModel: "primary", IntervalSeconds: 120}
	require.NoError(t, applyMonitorUpdate(m, ChannelMonitorUpdateParams{}))
	require.True(t, m.V3FusionEnabled)
	off := false
	require.NoError(t, applyMonitorUpdate(m, ChannelMonitorUpdateParams{V3FusionEnabled: &off}))
	require.False(t, m.V3FusionEnabled)
	oldHash := ChannelMonitorProbeTargetHash(m)
	m.Name = "renamed"
	require.Equal(t, oldHash, ChannelMonitorProbeTargetHash(m))
	m.Endpoint = "https://other.example.com"
	require.NotEqual(t, oldHash, ChannelMonitorProbeTargetHash(m))
}
