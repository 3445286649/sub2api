package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// 首页「满血保障」数据：从站长自建的公开降智监测站拉取各分组检测记录，
// 精简后经本站公开接口给首页展示。
//
// 设计要点：
//   - 对接监测站的公开接口 v1（只新增字段、不改含义），监测站改版不影响首页。
//   - 按需拉取 + 内存缓存，不跑后台定时任务；并发请求合并为一次上游调用。
//   - 只拼固定路径 /api/public/v1/groups，管理员无法借此让服务器请求任意地址。
//   - 只允许 HTTPS，校验解析后 IP，拒绝内网与本机地址（防 SSRF / DNS Rebinding）。
//   - 上游失败时回退到上一次成功的数据并标记 stale；超过上限后宣告不可用，
//     前端此时只展示静态文案，绝不展示过期太久的数字。
const (
	qualityMonitorPath          = "/api/public/v1/groups"
	qualityMonitorCacheTTL      = 5 * time.Minute
	qualityMonitorStaleLimit    = time.Hour
	qualityMonitorFetchTimeout  = 5 * time.Second
	qualityMonitorMaxBodyBytes  = 512 << 10
	qualityMonitorMaxHistory    = 60
	qualityMonitorMaxGroups     = 2
	qualityMonitorFlightKey     = "quality-monitor"
	qualityMonitorFailRetryWait = 30 * time.Second
)

// 对外的检测记录状态。上游的 incorrect（答错）统一映射为 failed（疑似降智）。
const (
	QualityStatusPassed = "passed"
	QualityStatusFailed = "failed"
	QualityStatusError  = "error"
)

// 上游 v1 接口的记录状态。
const (
	qualityUpstreamPassed    = "passed"
	qualityUpstreamIncorrect = "incorrect"
	qualityUpstreamError     = "error"
)

var (
	// ErrQualityMonitorDisabled 表示后台未配置监测地址。
	ErrQualityMonitorDisabled = errors.New("quality monitor is not configured")
	// ErrQualityMonitorUnavailable 表示拉取失败且没有足够新的缓存可用。
	ErrQualityMonitorUnavailable = errors.New("quality monitor data is unavailable")
)

// QualityMonitorPoint 是一次检测的精简记录。
type QualityMonitorPoint struct {
	Status     string    `json:"s"`
	FinishedAt time.Time `json:"t"`
}

// QualityMonitorGroup 是一个被监测的分组。
type QualityMonitorGroup struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Model           string     `json:"model"`
	ReasoningEffort string     `json:"reasoning_effort,omitempty"`
	IntervalSeconds int        `json:"interval_seconds"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	// PassRate = 通过 ÷（通过 + 疑似降智），接口异常不计入；没有有效结果时为 nil。
	PassRate     *float64              `json:"pass_rate"`
	Passed       int                   `json:"passed"`
	Failed       int                   `json:"failed"`
	Errored      int                   `json:"errored"`
	LatestStatus string                `json:"latest_status,omitempty"`
	History      []QualityMonitorPoint `json:"history"`
}

// QualityMonitorSnapshot 是返回给首页的完整数据。
type QualityMonitorSnapshot struct {
	DetailURL string                `json:"detail_url"`
	UpdatedAt time.Time             `json:"updated_at"`
	Stale     bool                  `json:"stale"`
	Groups    []QualityMonitorGroup `json:"groups"`
}

// 监测站 /api/public/v1/groups 的响应结构（只声明用到的字段）。
type qualityMonitorUpstream struct {
	APIVersion string                        `json:"api_version"`
	Source     string                        `json:"source"`
	Groups     []qualityMonitorUpstreamGroup `json:"groups"`
}

type qualityMonitorUpstreamGroup struct {
	ID              string                         `json:"id"`
	Name            string                         `json:"name"`
	Model           string                         `json:"model"`
	ReasoningEffort *string                        `json:"reasoning_effort"`
	IntervalSeconds *int                           `json:"interval_seconds"`
	NextRunAt       *time.Time                     `json:"next_run_at"`
	Records         []qualityMonitorUpstreamRecord `json:"records"`
}

type qualityMonitorUpstreamRecord struct {
	Status     string    `json:"status"`
	FinishedAt time.Time `json:"finished_at"`
}

// qualityMonitorFetcher 抽象出上游请求，便于单测替换。
type qualityMonitorFetcher func(ctx context.Context, endpoint string) ([]byte, error)

type qualityMonitorCache struct {
	baseURL   string
	snapshot  *QualityMonitorSnapshot
	fetchedAt time.Time
	failedAt  time.Time
}

// QualityMonitorService 提供首页满血监测数据。
type QualityMonitorService struct {
	settingRepo SettingRepository
	fetch       qualityMonitorFetcher
	now         func() time.Time

	flight singleflight.Group
	mu     sync.Mutex
	cache  qualityMonitorCache
}

// NewQualityMonitorService 创建满血监测服务。
func NewQualityMonitorService(settingRepo SettingRepository) *QualityMonitorService {
	return &QualityMonitorService{
		settingRepo: settingRepo,
		fetch:       fetchQualityMonitor,
		now:         time.Now,
	}
}

// NewQualityMonitorServiceForTest 允许其他包的测试替换上游请求，避免真实联网。
func NewQualityMonitorServiceForTest(settingRepo SettingRepository, fetch func(ctx context.Context, endpoint string) ([]byte, error)) *QualityMonitorService {
	svc := NewQualityMonitorService(settingRepo)
	svc.fetch = fetch
	return svc
}

// NormalizeQualityMonitorURL 校验并规范化监测站根地址。空字符串表示关闭。
func NormalizeQualityMonitorURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(trimmed, urlvalidator.ValidationOptions{})
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// GetSnapshot 返回首页展示用的监测数据。
//
// 返回 ErrQualityMonitorUnavailable 时仍会带回只含 DetailURL 的快照，
// 让首页在无数据时也能给出监测站链接。
func (s *QualityMonitorService) GetSnapshot(ctx context.Context) (*QualityMonitorSnapshot, error) {
	settings, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyQualityMonitorURL})
	if err != nil {
		return nil, fmt.Errorf("load quality monitor settings: %w", err)
	}

	baseURL, err := NormalizeQualityMonitorURL(settings[SettingKeyQualityMonitorURL])
	if err != nil || baseURL == "" {
		return nil, ErrQualityMonitorDisabled
	}

	now := s.now()
	s.mu.Lock()
	cached := s.cache
	s.mu.Unlock()

	// 地址变了，旧缓存作废
	if cached.baseURL != baseURL {
		cached = qualityMonitorCache{baseURL: baseURL}
	}

	fresh := cached.snapshot != nil && now.Sub(cached.fetchedAt) < qualityMonitorCacheTTL
	// 刚失败过就先别打上游，避免监测站故障时每个访客都触发一次超时
	recentlyFailed := !cached.failedAt.IsZero() && now.Sub(cached.failedAt) < qualityMonitorFailRetryWait

	if !fresh && !recentlyFailed {
		result, _, _ := s.flight.Do(qualityMonitorFlightKey+"|"+baseURL, func() (any, error) {
			return s.refresh(baseURL), nil
		})
		if updated, ok := result.(qualityMonitorCache); ok {
			cached = updated
		}
	}

	if cached.snapshot == nil || now.Sub(cached.fetchedAt) > qualityMonitorStaleLimit {
		return &QualityMonitorSnapshot{DetailURL: baseURL, Groups: []QualityMonitorGroup{}}, ErrQualityMonitorUnavailable
	}

	out := *cached.snapshot
	out.DetailURL = baseURL
	out.Stale = now.Sub(cached.fetchedAt) >= qualityMonitorCacheTTL
	return &out, nil
}

// refresh 拉取一次上游并更新缓存，返回更新后的缓存副本。
func (s *QualityMonitorService) refresh(baseURL string) qualityMonitorCache {
	// 与访客请求解耦：访客断开连接不应中断这次拉取，否则缓存永远填不上
	ctx, cancel := context.WithTimeout(context.Background(), qualityMonitorFetchTimeout)
	defer cancel()

	body, err := s.fetch(ctx, baseURL+qualityMonitorPath)
	var snapshot *QualityMonitorSnapshot
	if err == nil {
		snapshot, err = parseQualityMonitor(body)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache.baseURL != baseURL {
		s.cache = qualityMonitorCache{baseURL: baseURL}
	}
	now := s.now()
	if err != nil {
		slog.Warn("quality monitor fetch failed", "url", baseURL, "error", err)
		s.cache.failedAt = now
		return s.cache
	}
	s.cache.snapshot = snapshot
	s.cache.fetchedAt = now
	s.cache.failedAt = time.Time{}
	return s.cache
}

func fetchQualityMonitor(ctx context.Context, endpoint string) ([]byte, error) {
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:               qualityMonitorFetchTimeout,
		ResponseHeaderTimeout: qualityMonitorFetchTimeout,
		ValidateResolvedIP:    true,
		AllowPrivateHosts:     false,
	})
	if err != nil {
		return nil, fmt.Errorf("build http client: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, qualityMonitorMaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) > qualityMonitorMaxBodyBytes {
		return nil, errors.New("response body too large")
	}
	return body, nil
}

// parseQualityMonitor 把监测站原始响应精简为首页数据。
// 数据结构对不上、或监测站不是本站自测数据时返回错误，由调用方降级，而不是展示残缺或错误的数字。
func parseQualityMonitor(body []byte) (*QualityMonitorSnapshot, error) {
	var upstream qualityMonitorUpstream
	if err := json.Unmarshal(body, &upstream); err != nil {
		return nil, fmt.Errorf("decode monitor groups: %w", err)
	}
	if upstream.APIVersion != "v1" {
		return nil, fmt.Errorf("unexpected monitor api version %q", upstream.APIVersion)
	}
	// 监测站切到「API 拉取」模式时没有本站自测数据
	if upstream.Source != "self" {
		return nil, fmt.Errorf("monitor source is %q, not self", upstream.Source)
	}

	snapshot := &QualityMonitorSnapshot{Groups: make([]QualityMonitorGroup, 0, qualityMonitorMaxGroups)}
	for _, raw := range upstream.Groups {
		if len(snapshot.Groups) == qualityMonitorMaxGroups {
			break
		}
		group := summarizeQualityMonitorGroup(raw)
		if group.ID == "" {
			continue
		}
		if n := len(group.History); n > 0 && group.History[n-1].FinishedAt.After(snapshot.UpdatedAt) {
			snapshot.UpdatedAt = group.History[n-1].FinishedAt
		}
		snapshot.Groups = append(snapshot.Groups, group)
	}
	if len(snapshot.Groups) == 0 {
		return nil, errors.New("monitor has no enabled groups")
	}
	return snapshot, nil
}

func summarizeQualityMonitorGroup(raw qualityMonitorUpstreamGroup) QualityMonitorGroup {
	group := QualityMonitorGroup{
		ID:      strings.TrimSpace(raw.ID),
		Name:    strings.TrimSpace(raw.Name),
		Model:   strings.TrimSpace(raw.Model),
		History: []QualityMonitorPoint{},
	}
	if raw.ReasoningEffort != nil {
		group.ReasoningEffort = strings.TrimSpace(*raw.ReasoningEffort)
	}
	if raw.IntervalSeconds != nil && *raw.IntervalSeconds > 0 {
		group.IntervalSeconds = *raw.IntervalSeconds
	}
	if raw.NextRunAt != nil && !raw.NextRunAt.IsZero() {
		next := raw.NextRunAt.UTC()
		group.NextRunAt = &next
	}

	// 只保留已结束且状态可识别的记录；未知状态一律跳过，不猜测含义
	points := make([]QualityMonitorPoint, 0, len(raw.Records))
	for _, record := range raw.Records {
		if record.FinishedAt.IsZero() {
			continue
		}
		var status string
		switch strings.ToLower(strings.TrimSpace(record.Status)) {
		case qualityUpstreamPassed:
			status = QualityStatusPassed
		case qualityUpstreamIncorrect:
			status = QualityStatusFailed
		case qualityUpstreamError:
			status = QualityStatusError
		default:
			continue
		}
		points = append(points, QualityMonitorPoint{Status: status, FinishedAt: record.FinishedAt.UTC()})
	}
	if len(points) > qualityMonitorMaxHistory {
		points = points[len(points)-qualityMonitorMaxHistory:]
	}

	for _, point := range points {
		switch point.Status {
		case QualityStatusPassed:
			group.Passed++
		case QualityStatusFailed:
			group.Failed++
		case QualityStatusError:
			group.Errored++
		}
	}
	if valid := group.Passed + group.Failed; valid > 0 {
		rate := float64(group.Passed) / float64(valid)
		group.PassRate = &rate
	}
	if n := len(points); n > 0 {
		group.LatestStatus = points[n-1].Status
	}
	group.History = points
	return group
}
