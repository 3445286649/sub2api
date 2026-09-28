//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 截取自监测站 /api/monitors 的真实响应（记录条数已裁剪，字段保持原样）。
const qualityMonitorFixture = `{
  "source": "self",
  "candy": {
    "model": "gpt-6-astra",
    "reasoning_effort": "low",
    "interval_seconds": 1200,
    "next_run_at": "2026-09-28T06:36:55.683Z",
    "running": false,
    "history": [
      {"id": "a1", "status": "error", "finished_at": "2026-09-28T04:09:11.000Z", "duration_ms": 494, "error": "模型接口返回 HTTP 502", "has_html": false},
      {"id": "a2", "status": "passed", "finished_at": "2026-09-28T04:30:11.573Z", "duration_ms": 3036, "answer": "{\"answer\":8}", "output_tokens": 9, "has_html": false},
      {"id": "a3", "status": "passed", "finished_at": "2026-09-28T04:51:07.523Z", "duration_ms": 3297, "answer": "{\"answer\":8}", "output_tokens": 9, "has_html": false},
      {"id": "a4", "status": "failed", "finished_at": "2026-09-28T05:12:13.088Z", "duration_ms": 3290, "answer": "{\"answer\":7}", "output_tokens": 9, "has_html": false},
      {"id": "a5", "status": "passed", "finished_at": "2026-09-28T05:55:04.402Z", "duration_ms": 3195, "answer": "{\"answer\":8}", "output_tokens": 9, "has_html": false}
    ]
  },
  "pelican_monitor": {
    "model": "gpt-6-astra",
    "reasoning_effort": "low",
    "interval_seconds": 1200,
    "next_run_at": "2026-09-28T06:36:55.683Z",
    "running": false,
    "history": [
      {"id": "b1", "status": "passed", "finished_at": "2026-09-28T05:33:57.000Z", "duration_ms": 51230, "output_tokens": 3401, "has_html": true, "assessment": "{'quality': 'normal'}"},
      {"id": "b2", "status": "passed", "finished_at": "2026-09-28T05:55:57.643Z", "duration_ms": 53241, "output_tokens": 3517, "has_html": true, "assessment": "{'quality': 'normal'}"}
    ]
  }
}`

type qualityMonitorSettingRepo struct {
	mu     sync.Mutex
	values map[string]string
}

func (r *qualityMonitorSettingRepo) Get(context.Context, string) (*Setting, error) { return nil, nil }
func (r *qualityMonitorSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.values[key], nil
}
func (r *qualityMonitorSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}
func (r *qualityMonitorSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}
func (r *qualityMonitorSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *qualityMonitorSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (r *qualityMonitorSettingRepo) Delete(context.Context, string) error { return nil }

type qualityMonitorHarness struct {
	svc     *QualityMonitorService
	repo    *qualityMonitorSettingRepo
	now     time.Time
	calls   atomic.Int32
	body    []byte
	fail    error
	lastURL string
	mu      sync.Mutex
}

func newQualityMonitorHarness(t *testing.T) *qualityMonitorHarness {
	t.Helper()
	h := &qualityMonitorHarness{
		repo: &qualityMonitorSettingRepo{values: map[string]string{
			SettingKeyQualityMonitorURL:       "https://monitor.example.com/",
			SettingKeyQualityMonitorGroupName: "OpenAI 满血分组",
		}},
		now:  time.Date(2026, 9, 28, 6, 0, 0, 0, time.UTC),
		body: []byte(qualityMonitorFixture),
	}
	h.svc = NewQualityMonitorService(h.repo)
	h.svc.now = func() time.Time {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.now
	}
	h.svc.fetch = func(_ context.Context, endpoint string) ([]byte, error) {
		h.calls.Add(1)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.lastURL = endpoint
		if h.fail != nil {
			return nil, h.fail
		}
		return h.body, nil
	}
	return h
}

func (h *qualityMonitorHarness) advance(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = h.now.Add(d)
}

func (h *qualityMonitorHarness) setFail(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fail = err
}

func TestQualityMonitor_ParsesRealPayload(t *testing.T) {
	h := newQualityMonitorHarness(t)

	snap, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)

	require.Equal(t, "https://monitor.example.com/api/monitors", h.lastURL, "只请求固定路径")
	require.Equal(t, "OpenAI 满血分组", snap.GroupName)
	require.Equal(t, "https://monitor.example.com", snap.DetailURL)
	require.Equal(t, "gpt-6-astra", snap.Model)
	require.Equal(t, 1200, snap.IntervalSeconds)
	require.False(t, snap.Stale)
	require.Equal(t, time.Date(2026, 9, 28, 5, 55, 57, 643000000, time.UTC), snap.UpdatedAt)

	require.Len(t, snap.Tests, 2)
	logic := snap.Tests[0]
	require.Equal(t, "logic", logic.Key)
	require.Equal(t, 3, logic.Passed)
	require.Equal(t, 1, logic.Failed)
	require.Equal(t, 1, logic.Errored)
	require.NotNil(t, logic.PassRate)
	// 通过 ÷（通过 + 未通过），接口异常不计入
	require.InDelta(t, 0.75, *logic.PassRate, 1e-9)
	require.Len(t, logic.History, 5)
	require.Equal(t, QualityStatusError, logic.History[0].Status)

	creative := snap.Tests[1]
	require.Equal(t, "creative", creative.Key)
	require.InDelta(t, 1.0, *creative.PassRate, 1e-9)
}

func TestQualityMonitor_DisabledWhenURLMissingOrInvalid(t *testing.T) {
	for _, raw := range []string{"", "   ", "http://monitor.example.com", "https://127.0.0.1", "https://localhost", "not a url"} {
		h := newQualityMonitorHarness(t)
		h.repo.values[SettingKeyQualityMonitorURL] = raw

		_, err := h.svc.GetSnapshot(context.Background())
		require.ErrorIs(t, err, ErrQualityMonitorDisabled, raw)
		require.Zero(t, h.calls.Load(), "未配置或地址非法时不应发起请求: %q", raw)
	}
}

func TestQualityMonitor_CachesWithinTTL(t *testing.T) {
	h := newQualityMonitorHarness(t)

	_, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	h.advance(qualityMonitorCacheTTL - time.Second)
	_, err = h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, h.calls.Load())

	h.advance(2 * time.Second)
	_, err = h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, h.calls.Load())
}

func TestQualityMonitor_ConcurrentRequestsShareOneFetch(t *testing.T) {
	h := newQualityMonitorHarness(t)
	release := make(chan struct{})
	inner := h.svc.fetch
	h.svc.fetch = func(ctx context.Context, endpoint string) ([]byte, error) {
		<-release
		return inner(ctx, endpoint)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = h.svc.GetSnapshot(context.Background())
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	require.EqualValues(t, 1, h.calls.Load())
}

func TestQualityMonitor_FallsBackToStaleThenUnavailable(t *testing.T) {
	h := newQualityMonitorHarness(t)
	_, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)

	h.setFail(errors.New("upstream down"))
	h.advance(qualityMonitorCacheTTL + time.Minute)

	snap, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.True(t, snap.Stale, "上游失败时回退到旧数据并标记 stale")

	// 失败后的冷却期内不再打上游
	calls := h.calls.Load()
	_, _ = h.svc.GetSnapshot(context.Background())
	require.Equal(t, calls, h.calls.Load())

	h.advance(qualityMonitorStaleLimit)
	snap, err = h.svc.GetSnapshot(context.Background())
	require.ErrorIs(t, err, ErrQualityMonitorUnavailable, "超过上限后不再展示旧数字")
	require.Equal(t, "https://monitor.example.com", snap.DetailURL, "无数据时仍给出监测站链接")
	require.Empty(t, snap.Tests, "无数据时不带任何检测结果")
}

func TestQualityMonitor_RejectsUnexpectedPayload(t *testing.T) {
	for _, body := range []string{`{}`, `{"candy":{"history":[]}}`, `not json`, `[]`} {
		h := newQualityMonitorHarness(t)
		h.body = []byte(body)

		_, err := h.svc.GetSnapshot(context.Background())
		require.ErrorIs(t, err, ErrQualityMonitorUnavailable, body)
	}
}

func TestQualityMonitor_URLChangeInvalidatesCache(t *testing.T) {
	h := newQualityMonitorHarness(t)
	_, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)

	h.repo.values[SettingKeyQualityMonitorURL] = "https://other.example.com"
	snap, err := h.svc.GetSnapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, h.calls.Load())
	require.Equal(t, "https://other.example.com", snap.DetailURL)
}

func TestSummarizeQualityMonitor_SkipsUnknownAndCapsHistory(t *testing.T) {
	base := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	records := []qualityMonitorUpstreamRecord{
		{Status: "running", FinishedAt: base},
		{Status: "passed"},
	}
	for i := 0; i < qualityMonitorMaxHistory+5; i++ {
		records = append(records, qualityMonitorUpstreamRecord{Status: "PASSED", FinishedAt: base.Add(time.Duration(i) * time.Minute)})
	}

	test := summarizeQualityMonitor("logic", records)
	require.Len(t, test.History, qualityMonitorMaxHistory)
	require.Equal(t, qualityMonitorMaxHistory, test.Passed)
	require.Equal(t, base.Add(time.Duration(qualityMonitorMaxHistory+4)*time.Minute), test.History[len(test.History)-1].FinishedAt)

	empty := summarizeQualityMonitor("logic", []qualityMonitorUpstreamRecord{{Status: "error", FinishedAt: base}})
	require.Nil(t, empty.PassRate, "只有接口异常时通过率为空，而不是 0%")
}

func TestNormalizeQualityMonitorURL(t *testing.T) {
	got, err := NormalizeQualityMonitorURL(" https://manxue.codeplan.codes/ ")
	require.NoError(t, err)
	require.Equal(t, "https://manxue.codeplan.codes", got)

	got, err = NormalizeQualityMonitorURL("")
	require.NoError(t, err)
	require.Empty(t, got)

	for _, raw := range []string{"http://example.com", "https://10.0.0.1", "ftp://example.com"} {
		_, err := NormalizeQualityMonitorURL(raw)
		require.Error(t, err, raw)
	}
}
