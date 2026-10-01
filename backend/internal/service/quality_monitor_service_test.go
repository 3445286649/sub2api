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

// 按监测站 /api/public/v1/groups 的真实响应裁剪（字段保持原样）。
const qualityMonitorFixture = `{
  "api_version": "v1",
  "source": "self",
  "question_version": "candy-shapes-21",
  "generated_at": "2026-09-28T06:00:00.000Z",
  "groups": [
    {
      "id": "channel-1",
      "name": "Pro尊享分组",
      "model": "gpt-6-astra",
      "reasoning_effort": "medium",
      "interval_seconds": 1200,
      "next_run_at": "2026-09-28T06:15:04.000Z",
      "running": false,
      "stats": {"total": 5, "valid": 4, "passed": 3, "incorrect": 1, "errors": 1, "pass_rate": 75, "latest_status": "passed", "latest_finished_at": "2026-09-28T05:55:04.402Z"},
      "records": [
        {"id": "a1", "status": "error", "finished_at": "2026-09-28T04:09:11.000Z", "duration_ms": 494, "attempts": 3, "parsed_answer": null, "input_tokens": null, "output_tokens": null, "reasoning_tokens": null, "tps": null},
        {"id": "a2", "status": "passed", "finished_at": "2026-09-28T04:30:11.573Z", "duration_ms": 31403, "attempts": 1, "parsed_answer": 21, "input_tokens": 4625, "output_tokens": 944, "reasoning_tokens": 454, "tps": 30.1},
        {"id": "a3", "status": "passed", "finished_at": "2026-09-28T04:51:07.523Z", "duration_ms": 30111, "attempts": 1, "parsed_answer": 21, "input_tokens": 4625, "output_tokens": 912, "reasoning_tokens": 440, "tps": 30.3},
        {"id": "a4", "status": "incorrect", "finished_at": "2026-09-28T05:12:13.088Z", "duration_ms": 29870, "attempts": 1, "parsed_answer": 19, "input_tokens": 4625, "output_tokens": 870, "reasoning_tokens": 410, "tps": 29.1},
        {"id": "a5", "status": "passed", "finished_at": "2026-09-28T05:55:04.402Z", "duration_ms": 31490, "attempts": 1, "parsed_answer": 21, "input_tokens": 4625, "output_tokens": 914, "reasoning_tokens": 428, "tps": 29}
      ]
    },
    {
      "id": "channel-2",
      "name": "Pro独享分组",
      "model": "gpt-6-astra",
      "reasoning_effort": null,
      "interval_seconds": null,
      "next_run_at": null,
      "running": false,
      "stats": {"total": 2, "valid": 2, "passed": 2, "incorrect": 0, "errors": 0, "pass_rate": 100, "latest_status": "passed", "latest_finished_at": "2026-09-28T05:55:57.643Z"},
      "records": [
        {"id": "b1", "status": "passed", "finished_at": "2026-09-28T05:33:57.000Z", "duration_ms": 37200, "attempts": 1, "parsed_answer": 21},
        {"id": "b2", "status": "passed", "finished_at": "2026-09-28T05:55:57.643Z", "duration_ms": 37100, "attempts": 1, "parsed_answer": 21}
      ]
    }
  ]
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
			SettingKeyQualityMonitorURL: "https://monitor.example.com/",
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

	require.Equal(t, "https://monitor.example.com/api/public/v1/groups", h.lastURL, "只请求固定路径")
	require.Equal(t, "https://monitor.example.com", snap.DetailURL)
	require.False(t, snap.Stale)
	require.Equal(t, time.Date(2026, 9, 28, 5, 55, 57, 643000000, time.UTC), snap.UpdatedAt)

	require.Len(t, snap.Groups, 2)
	first := snap.Groups[0]
	require.Equal(t, "channel-1", first.ID)
	require.Equal(t, "Pro尊享分组", first.Name)
	require.Equal(t, "gpt-6-astra", first.Model)
	require.Equal(t, "medium", first.ReasoningEffort)
	require.Equal(t, 1200, first.IntervalSeconds)
	require.NotNil(t, first.NextRunAt)
	require.Equal(t, 3, first.Passed)
	require.Equal(t, 1, first.Failed, "incorrect 映射为疑似降智")
	require.Equal(t, 1, first.Errored)
	require.NotNil(t, first.PassRate)
	// 通过 ÷（通过 + 疑似降智），接口异常不计入
	require.InDelta(t, 0.75, *first.PassRate, 1e-9)
	require.Equal(t, QualityStatusPassed, first.LatestStatus)
	require.Len(t, first.History, 5)
	require.Equal(t, QualityStatusError, first.History[0].Status)
	require.Equal(t, QualityStatusFailed, first.History[3].Status)

	second := snap.Groups[1]
	require.Equal(t, "Pro独享分组", second.Name)
	require.Empty(t, second.ReasoningEffort, "null 推理程度不展示")
	require.Zero(t, second.IntervalSeconds, "定时检测未启用")
	require.Nil(t, second.NextRunAt)
	require.InDelta(t, 1.0, *second.PassRate, 1e-9)
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
	require.Empty(t, snap.Groups, "无数据时不带任何检测结果")
}

func TestQualityMonitor_RejectsUnexpectedPayload(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`not json`,
		`[]`,
		// 旧版 /api/monitors 格式
		`{"source":"self","candy":{"history":[]}}`,
		// 监测站切到 API 拉取模式，没有本站自测数据
		`{"api_version":"v1","source":"api","groups":[]}`,
		`{"api_version":"v1","source":"self","groups":[]}`,
		`{"api_version":"v2","source":"self","groups":[{"id":"channel-1","records":[]}]}`,
	} {
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

func TestSummarizeQualityMonitorGroup_SkipsUnknownAndCapsHistory(t *testing.T) {
	base := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	records := []qualityMonitorUpstreamRecord{
		{Status: "running", FinishedAt: base},
		{Status: "passed"},
	}
	for i := 0; i < qualityMonitorMaxHistory+5; i++ {
		records = append(records, qualityMonitorUpstreamRecord{Status: "PASSED", FinishedAt: base.Add(time.Duration(i) * time.Minute)})
	}

	group := summarizeQualityMonitorGroup(qualityMonitorUpstreamGroup{ID: "channel-1", Records: records})
	require.Len(t, group.History, qualityMonitorMaxHistory)
	require.Equal(t, qualityMonitorMaxHistory, group.Passed)
	require.Equal(t, base.Add(time.Duration(qualityMonitorMaxHistory+4)*time.Minute), group.History[len(group.History)-1].FinishedAt)

	empty := summarizeQualityMonitorGroup(qualityMonitorUpstreamGroup{ID: "channel-1", Records: []qualityMonitorUpstreamRecord{{Status: "error", FinishedAt: base}}})
	require.Nil(t, empty.PassRate, "只有接口异常时通过率为空，而不是 0%")
	require.Equal(t, QualityStatusError, empty.LatestStatus)
}

func TestParseQualityMonitor_CapsGroups(t *testing.T) {
	body := `{"api_version":"v1","source":"self","groups":[
		{"id":"channel-1","records":[]},{"id":"channel-2","records":[]},{"id":"channel-3","records":[]}]}`
	snap, err := parseQualityMonitor([]byte(body))
	require.NoError(t, err)
	require.Len(t, snap.Groups, qualityMonitorMaxGroups)
	require.Empty(t, snap.Groups[0].History, "暂无记录的分组照常返回，前端显示空占位")
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
