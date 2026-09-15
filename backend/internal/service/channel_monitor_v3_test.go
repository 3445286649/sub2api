package service

import "testing"

func TestChannelMonitorV3Runtime(t *testing.T) {
	if normalizeChannelMonitorMode(" V3 ") != ChannelMonitorModeV3 {
		t.Fatal("V3 must remain distinct")
	}
	runtime := ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV3}
	if runtime.ActiveProbesAllowed() || !runtime.PassiveAggregationAllowed() {
		t.Fatal("V3 must only aggregate")
	}
	runtime.Enabled = false
	if runtime.PassiveAggregationAllowed() {
		t.Fatal("disabled V3 aggregated")
	}
}

func TestChannelMonitorV3AssociationUpdate(t *testing.T) {
	id := int64(9)
	monitor := &ChannelMonitor{V3GroupID: &id, Provider: MonitorProviderOpenAI, Endpoint: "https://example.com", APIKey: "test", PrimaryModel: "model", IntervalSeconds: 60}
	if err := applyMonitorUpdate(monitor, ChannelMonitorUpdateParams{}); err != nil || *monitor.V3GroupID != 9 {
		t.Fatal("omitted association must remain")
	}
	zero := int64(0)
	if err := applyMonitorUpdate(monitor, ChannelMonitorUpdateParams{V3GroupID: &zero}); err != nil || monitor.V3GroupID != nil {
		t.Fatal("zero must clear association")
	}
}
