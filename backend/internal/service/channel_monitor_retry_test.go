//go:build unit

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRunCheckForModelWithSequentialProbeStopsAfterFirstSuccess(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModelWithSequentialProbe(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 1 {
		t.Fatalf("expected one logical attempt, got %d", res.Attempts)
	}
	if h.callCount != 1 {
		t.Fatalf("expected one upstream call, got %d", h.callCount)
	}
}

func TestRunCheckForModelWithSequentialProbeSucceedsOnSecondAttempt(t *testing.T) {
	h := &openAICaptureHandler{statusSequence: []int{http.StatusBadRequest, http.StatusOK}}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModelWithSequentialProbe(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after fallback attempt, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 2 {
		t.Fatalf("expected two logical attempts, got %d", res.Attempts)
	}
	if h.callCount != 2 {
		t.Fatalf("expected two upstream calls, got %d", h.callCount)
	}
	if res.Message == "" || !containsAll(res.Message, "第 2 次", "前 1 次") {
		t.Fatalf("expected retry success summary, got %q", res.Message)
	}
}

func TestRunCheckForModelWithSequentialProbeExcludesSelectedAccountOnRetry(t *testing.T) {
	h := &openAICaptureHandler{statusSequence: []int{http.StatusBadRequest, http.StatusOK}}
	endpoint := setupFakeOpenAI(t, h)
	signer := NewChannelMonitorSigner()

	res := runCheckForModelWithSequentialProbeSigner(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil, signer)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after retry, got %s", res.Status)
	}
	if len(h.seenHeaders) < 2 {
		t.Fatalf("expected at least two requests, got %d", len(h.seenHeaders))
	}
	if got := h.seenHeaders[1].Get(ChannelMonitorHeaderExcludeAccounts); got != "774" {
		t.Fatalf("expected second attempt to exclude account 774, got %q", got)
	}
	if h.seenHeaders[1].Get(ChannelMonitorHeaderSignature) == "" {
		t.Fatal("expected signed monitor retry header")
	}
}

func TestChannelMonitorServiceOnlySignsSameOriginEndpoint(t *testing.T) {
	svc := NewChannelMonitorService(nil, nil)
	svc.SetSettingReader(channelMonitorSettingReaderStub{apiBaseURL: "https://api.example.com"})

	if svc.signerForEndpoint(context.Background(), "https://api.example.com") == nil {
		t.Fatal("expected same-origin endpoint to use monitor signer")
	}
	if svc.signerForEndpoint(context.Background(), "https://upstream.example.com") != nil {
		t.Fatal("expected external endpoint not to use monitor signer")
	}
}

func TestRunCheckForModelWithSequentialProbeFailsAfterThreeAttempts(t *testing.T) {
	h := &openAICaptureHandler{statusSequence: []int{http.StatusBadRequest, http.StatusBadRequest, http.StatusBadRequest}}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModelWithSequentialProbe(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("expected final error, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 3 {
		t.Fatalf("expected three logical attempts, got %d", res.Attempts)
	}
	if h.callCount != 3 {
		t.Fatalf("expected three upstream calls, got %d", h.callCount)
	}
	if res.Message == "" || !containsAll(res.Message, "连续 3 次") {
		t.Fatalf("expected exhausted summary, got %q", res.Message)
	}
}

func TestRunCheckForModelWithSequentialProbeKeepsDegradedWhenNoSuccess(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModelWithSequentialProbe(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil, func(r *CheckResult) {
		latency := int((monitorDegradedThreshold + 1).Milliseconds())
		r.Status = MonitorStatusDegraded
		r.LatencyMs = &latency
		r.Message = "slow response"
	})

	if res.Status != MonitorStatusDegraded {
		t.Fatalf("expected degraded, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 3 {
		t.Fatalf("expected three logical attempts for repeated degraded results, got %d", res.Attempts)
	}
}

type channelMonitorSettingReaderStub struct {
	apiBaseURL string
}

func (s channelMonitorSettingReaderStub) GetAllSettings(context.Context) (*SystemSettings, error) {
	return &SystemSettings{APIBaseURL: s.apiBaseURL}, nil
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
