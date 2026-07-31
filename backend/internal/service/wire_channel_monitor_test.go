package service

import "testing"

func TestProvideChannelMonitorServiceWiresSettingReader(t *testing.T) {
	settingService := &SettingService{}
	svc := ProvideChannelMonitorService(nil, nil, settingService)

	if svc == nil {
		t.Fatal("expected channel monitor service")
	}
	if svc.settingReader != settingService {
		t.Fatal("expected channel monitor setting reader to be wired")
	}
}
