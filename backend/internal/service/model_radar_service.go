package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	ErrModelRadarDisabled      = infraerrors.Forbidden("MODEL_RADAR_DISABLED", "model radar is disabled")
	ErrModelRadarNotConfigured = infraerrors.BadRequest("MODEL_RADAR_NOT_CONFIGURED", "model radar API key is not configured")
	ErrModelRadarRunNotFound   = infraerrors.NotFound("MODEL_RADAR_RUN_NOT_FOUND", "model radar run not found")
	ErrModelRadarAPIKeyInvalid = infraerrors.BadRequest("MODEL_RADAR_API_KEY_INVALID", "selected model radar API key is invalid")

	modelRadarSecretPattern = regexp.MustCompile(`(?i)\b(sk-[a-z0-9][a-z0-9._-]{8,})\b`)
)

type modelRadarHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type modelRadarAPIKeyReader interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type ModelRadarService struct {
	repo           ModelRadarRepository
	settingRepo    SettingRepository
	settingService *SettingService
	encryptor      SecretEncryptor
	apiKeyReader   modelRadarAPIKeyReader
	httpClient     modelRadarHTTPClient
}

func NewModelRadarService(repo ModelRadarRepository, settingRepo SettingRepository, settingService *SettingService, encryptor SecretEncryptor, apiKeyReader modelRadarAPIKeyReader) *ModelRadarService {
	return &ModelRadarService{
		repo:           repo,
		settingRepo:    settingRepo,
		settingService: settingService,
		encryptor:      encryptor,
		apiKeyReader:   apiKeyReader,
		httpClient:     &http.Client{Timeout: time.Duration(modelRadarDefaultTimeoutSec) * time.Second},
	}
}

func (s *ModelRadarService) SetHTTPClient(client modelRadarHTTPClient) {
	if client != nil {
		s.httpClient = client
	}
}

func (s *ModelRadarService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyModelRadarEnabled)
	return err == nil && value == "true"
}

func (s *ModelRadarService) GetPublicCurrent(ctx context.Context) (*ModelRadarCurrent, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrModelRadarDisabled
	}
	return s.Current(ctx)
}

func (s *ModelRadarService) Current(ctx context.Context) (*ModelRadarCurrent, error) {
	run, err := s.repo.GetLatestPublishedRun(ctx)
	if err != nil {
		if errors.Is(err, ErrModelRadarRunNotFound) {
			return &ModelRadarCurrent{Results: []*ModelRadarResult{}, History: []*ModelRadarResult{}}, nil
		}
		return nil, err
	}
	results, err := s.repo.ListResults(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	sortModelRadarResults(results)
	history, _ := s.repo.ListPublishedBestResults(ctx, 7)
	var recommendation *ModelRadarResult
	if len(results) > 0 {
		recommendation = results[0]
	}
	updated := run.FinishedAt
	if updated == nil {
		updated = &run.UpdatedAt
	}
	return &ModelRadarCurrent{
		Run:            run,
		Recommendation: recommendation,
		Results:        results,
		History:        history,
		UpdatedAt:      updated,
	}, nil
}

func (s *ModelRadarService) GetConfig(ctx context.Context) (*ModelRadarConfig, error) {
	cfg, err := s.loadConfig(ctx, false)
	if err != nil {
		return nil, err
	}
	cfg.APIKey = ""
	return cfg, nil
}

func (s *ModelRadarService) UpdateConfig(ctx context.Context, cfg ModelRadarConfig) (*ModelRadarConfig, error) {
	return s.UpdateConfigWithOptions(ctx, cfg, ModelRadarConfigUpdateOptions{})
}

func (s *ModelRadarService) UpdateConfigWithOptions(ctx context.Context, cfg ModelRadarConfig, opts ModelRadarConfigUpdateOptions) (*ModelRadarConfig, error) {
	existing, _ := s.loadConfig(ctx, true)
	cfg = normalizeModelRadarConfig(cfg)
	if cfg.APIKeySource == ModelRadarAPIKeySourceExisting {
		if cfg.APIKeyID == nil || *cfg.APIKeyID <= 0 {
			return nil, ErrModelRadarAPIKeyInvalid
		}
		key, err := s.loadExistingAPIKey(ctx, *cfg.APIKeyID)
		if err != nil {
			return nil, err
		}
		if opts.AdminUserID > 0 && key.UserID != opts.AdminUserID {
			return nil, ErrModelRadarAPIKeyInvalid
		}
		if err := validateModelRadarExistingAPIKey(key); err != nil {
			return nil, err
		}
		cfg.APIKey = ""
		cfg.APIKeyConfigured = true
		cfg.APIKeyMasked = maskModelRadarKey(key.Key)
		cfg.APIKeyName = key.Name
		cfg.APIKeyStatus = key.Status
		if key.Group != nil {
			cfg.APIKeyGroupName = key.Group.Name
		}
	} else {
		cfg.APIKeyID = nil
		cfg.APIKeyName = ""
		cfg.APIKeyGroupName = ""
		cfg.APIKeyStatus = ""
		if strings.TrimSpace(cfg.APIKey) == "" && existing != nil && existing.APIKeySource == ModelRadarAPIKeySourceCustom {
			cfg.APIKey = existing.APIKey
		}
		if strings.TrimSpace(cfg.APIKey) != "" {
			encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(cfg.APIKey))
			if err != nil {
				return nil, fmt.Errorf("encrypt model radar api key: %w", err)
			}
			cfg.APIKey = encrypted
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal model radar config: %w", err)
	}
	updates := map[string]string{
		SettingKeyModelRadarEnabled: fmt.Sprintf("%t", cfg.Enabled),
		SettingKeyModelRadarConfig:  string(raw),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, err
	}
	return s.GetConfig(ctx)
}

func (s *ModelRadarService) RunNow(ctx context.Context) (*ModelRadarRunDetail, error) {
	run, cfg, combinations, err := s.prepareRun(ctx, ModelRadarRunParams{TriggerType: ModelRadarTriggerManual, Now: time.Now()})
	if err != nil {
		return nil, err
	}
	s.startBackgroundRun(run, cfg, combinations)
	return &ModelRadarRunDetail{
		Run:         run,
		Results:     []*ModelRadarResult{},
		TaskResults: map[int64][]*ModelRadarTaskResult{},
	}, nil
}

func (s *ModelRadarService) Run(ctx context.Context, params ModelRadarRunParams) (*ModelRadarRunDetail, error) {
	run, cfg, combinations, err := s.prepareRun(ctx, params)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return s.Detail(ctx, run.ID)
	}
	if err := s.completeRun(ctx, run, cfg, combinations); err != nil {
		return nil, err
	}
	return s.Detail(ctx, run.ID)
}

func (s *ModelRadarService) prepareRun(ctx context.Context, params ModelRadarRunParams) (*ModelRadarRun, *ModelRadarConfig, []ModelRadarCombination, error) {
	if params.Now.IsZero() {
		params.Now = time.Now()
	}
	if params.TriggerType == "" {
		params.TriggerType = ModelRadarTriggerScheduled
	}
	cfg, err := s.loadConfig(ctx, true)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := s.resolveAPIKeyForRun(ctx, cfg); err != nil {
		return nil, nil, nil, err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, nil, nil, ErrModelRadarNotConfigured
	}
	combinations := enabledModelRadarMatrix(cfg.Matrix)
	if len(combinations) == 0 {
		return nil, nil, nil, infraerrors.BadRequest("MODEL_RADAR_EMPTY_MATRIX", "model radar matrix is empty")
	}
	runDate := time.Date(params.Now.In(modelRadarLocation()).Year(), params.Now.In(modelRadarLocation()).Month(), params.Now.In(modelRadarLocation()).Day(), 0, 0, 0, 0, time.UTC)
	if params.TriggerType == ModelRadarTriggerScheduled {
		existing, err := s.repo.GetScheduledRunByDate(ctx, runDate)
		if err == nil {
			return existing, nil, nil, nil
		}
	}
	run := &ModelRadarRun{
		RunDate:           runDate,
		TriggerType:       params.TriggerType,
		Status:            ModelRadarStatusRunning,
		TotalCombinations: len(combinations),
	}
	now := time.Now().UTC()
	run.StartedAt = &now
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, nil, nil, err
	}
	return run, cfg, combinations, nil
}

func (s *ModelRadarService) completeRun(ctx context.Context, run *ModelRadarRun, cfg *ModelRadarConfig, combinations []ModelRadarCombination) error {
	results := s.runMatrix(ctx, cfg, run.ID, combinations)
	sortModelRadarTaskCarriers(results)
	success := 0
	for i, item := range results {
		item.Result.Rank = i + 1
		if item.Result.Status == ModelRadarStatusSucceeded {
			success++
		}
		if err := s.repo.InsertResult(ctx, item.Result, item.Tasks); err != nil {
			return err
		}
	}
	finished := time.Now().UTC()
	run.FinishedAt = &finished
	run.SuccessCombinations = success
	run.Status = ModelRadarStatusSucceeded
	run.Published = success > 0
	if success == 0 {
		run.Status = ModelRadarStatusFailed
		run.Published = false
		run.ErrorMessage = "all model radar combinations failed"
	}
	return s.repo.UpdateRun(ctx, run)
}

func (s *ModelRadarService) startBackgroundRun(run *ModelRadarRun, cfg *ModelRadarConfig, combinations []ModelRadarCombination) {
	runCopy := *run
	cfgCopy := *cfg
	comboCopy := append([]ModelRadarCombination(nil), combinations...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.completeRun(ctx, &runCopy, &cfgCopy, comboCopy); err != nil {
			slog.Warn("model_radar: manual run failed", "run_id", runCopy.ID, "error", err)
			s.markRunFailed(runCopy.ID, err)
		}
	}()
}

func (s *ModelRadarService) markRunFailed(runID int64, runErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		slog.Warn("model_radar: load failed run for status update failed", "run_id", runID, "error", err)
		return
	}
	finished := time.Now().UTC()
	run.Status = ModelRadarStatusFailed
	run.Published = false
	run.FinishedAt = &finished
	run.ErrorMessage = sanitizeModelRadarError(runErr)
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		slog.Warn("model_radar: failed run status update failed", "run_id", runID, "error", err)
	}
}

func (s *ModelRadarService) Detail(ctx context.Context, id int64) (*ModelRadarRunDetail, error) {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.ListResults(ctx, id)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	tasks, err := s.repo.ListTaskResults(ctx, ids)
	if err != nil {
		return nil, err
	}
	return &ModelRadarRunDetail{Run: run, Results: results, TaskResults: tasks}, nil
}

func (s *ModelRadarService) ListRuns(ctx context.Context, limit int) ([]*ModelRadarRun, error) {
	return s.repo.ListRuns(ctx, limit)
}

type modelRadarTaskCarrier struct {
	Result *ModelRadarResult
	Tasks  []*ModelRadarTaskResult
}

func (s *ModelRadarService) runMatrix(ctx context.Context, cfg *ModelRadarConfig, runID int64, combinations []ModelRadarCombination) []modelRadarTaskCarrier {
	sem := make(chan struct{}, cfg.Concurrency)
	results := make([]modelRadarTaskCarrier, len(combinations))
	var g errgroup.Group
	for i, combo := range combinations {
		i, combo := i, combo
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			result, tasks := s.runCombination(ctx, cfg, runID, combo)
			results[i] = modelRadarTaskCarrier{Result: result, Tasks: tasks}
			return nil
		})
	}
	_ = g.Wait()
	return results
}

func (s *ModelRadarService) runCombination(ctx context.Context, cfg *ModelRadarConfig, runID int64, combo ModelRadarCombination) (*ModelRadarResult, []*ModelRadarTaskResult) {
	tasks := modelRadarTaskSet()
	taskResults := make([]*ModelRadarTaskResult, 0, len(tasks))
	passCount := 0
	errorCount := 0
	totalLatency := 0
	latencySamples := 0
	for _, task := range tasks {
		answer, latency, err := s.callModel(ctx, cfg, combo, task.Prompt)
		tr := &ModelRadarTaskResult{
			TaskID:         task.ID,
			TaskVersion:    task.Version,
			ExpectedAnswer: task.Expected,
			ActualAnswer:   truncateModelRadarAnswer(answer),
		}
		if latency > 0 {
			ms := int(latency / time.Millisecond)
			tr.LatencyMs = &ms
			totalLatency += ms
			latencySamples++
		}
		if err != nil {
			tr.ErrorMessage = sanitizeModelRadarError(err)
			errorCount++
		} else {
			tr.Passed = evaluateModelRadarAnswer(task, answer)
			if tr.Passed {
				passCount++
			}
		}
		taskResults = append(taskResults, tr)
	}
	var avg *int
	if latencySamples > 0 {
		v := totalLatency / latencySamples
		avg = &v
	}
	score := calculateModelRadarScore(taskResults, avg)
	status := ModelRadarStatusSucceeded
	if passCount == 0 && errorCount == len(tasks) {
		status = ModelRadarStatusFailed
	}
	result := &ModelRadarResult{
		RunID:           runID,
		Model:           combo.Model,
		ReasoningEffort: combo.ReasoningEffort,
		Score:           score,
		PassCount:       passCount,
		TotalCount:      len(tasks),
		AvgLatencyMs:    avg,
		ErrorCount:      errorCount,
		Status:          status,
	}
	return result, taskResults
}

func (s *ModelRadarService) callModel(ctx context.Context, cfg *ModelRadarConfig, combo ModelRadarCombination, prompt string) (string, time.Duration, error) {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(modelRadarDefaultTimeoutSec) * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body := map[string]any{
		"model": combo.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "Answer with the exact final answer only. Do not explain."},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  64,
		"temperature": 0,
	}
	if combo.ReasoningEffort != "" {
		body["reasoning_effort"] = combo.ReasoningEffort
		body["reasoning"] = map[string]any{"effort": combo.ReasoningEffort}
	}
	payload, _ := json.Marshal(body)
	base := normalizeModelRadarGatewayBaseURL(cfg.APIBaseURL)
	if base == "" && s.settingService != nil {
		base = normalizeModelRadarGatewayBaseURL(s.settingService.GetFrontendURL(ctx))
	}
	if base == "" {
		return "", 0, ErrModelRadarNotConfigured
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, base+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return "", latency, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", latency, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", latency, err
	}
	if len(out.Choices) == 0 {
		return "", latency, errors.New("empty choices")
	}
	return out.Choices[0].Message.Content, latency, nil
}

func (s *ModelRadarService) loadConfig(ctx context.Context, includePlainKey bool) (*ModelRadarConfig, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyModelRadarEnabled, SettingKeyModelRadarConfig, SettingKeyAPIBaseURL})
	if err != nil {
		return nil, err
	}
	cfg := defaultModelRadarConfig()
	cfg.Enabled = values[SettingKeyModelRadarEnabled] == "true"
	if raw := strings.TrimSpace(values[SettingKeyModelRadarConfig]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
		cfg.Enabled = values[SettingKeyModelRadarEnabled] == "true"
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = strings.TrimSpace(values[SettingKeyAPIBaseURL])
	}
	cfg = normalizeModelRadarConfig(cfg)
	if cfg.APIKeySource == ModelRadarAPIKeySourceExisting {
		if cfg.APIKeyID != nil && *cfg.APIKeyID > 0 {
			if key, err := s.loadExistingAPIKey(ctx, *cfg.APIKeyID); err == nil {
				cfg.APIKeyConfigured = true
				cfg.APIKeyMasked = maskModelRadarKey(key.Key)
				cfg.APIKeyName = key.Name
				cfg.APIKeyStatus = key.Status
				if key.Group != nil {
					cfg.APIKeyGroupName = key.Group.Name
				}
			} else {
				cfg.APIKeyConfigured = true
				cfg.APIKeyMasked = "读取失败"
				cfg.APIKeyStatus = "unavailable"
				cfg.APIKey = ""
			}
		}
	} else if strings.TrimSpace(cfg.APIKey) != "" {
		plain, err := s.encryptor.Decrypt(cfg.APIKey)
		if err == nil {
			cfg.APIKeyConfigured = true
			cfg.APIKeyMasked = maskModelRadarKey(plain)
			if includePlainKey {
				cfg.APIKey = plain
			} else {
				cfg.APIKey = ""
			}
		} else {
			cfg.APIKeyConfigured = true
			cfg.APIKeyMasked = "解密失败"
			cfg.APIKey = ""
			slog.Warn("model_radar: decrypt api key failed", "error", err)
		}
	}
	return &cfg, nil
}

func (s *ModelRadarService) resolveAPIKeyForRun(ctx context.Context, cfg *ModelRadarConfig) error {
	if cfg == nil || cfg.APIKeySource != ModelRadarAPIKeySourceExisting {
		return nil
	}
	if cfg.APIKeyID == nil || *cfg.APIKeyID <= 0 {
		return ErrModelRadarAPIKeyInvalid
	}
	key, err := s.loadExistingAPIKey(ctx, *cfg.APIKeyID)
	if err != nil {
		return err
	}
	if err := validateModelRadarExistingAPIKey(key); err != nil {
		return err
	}
	cfg.APIKey = key.Key
	cfg.APIKeyConfigured = true
	cfg.APIKeyMasked = maskModelRadarKey(key.Key)
	cfg.APIKeyName = key.Name
	cfg.APIKeyStatus = key.Status
	if key.Group != nil {
		cfg.APIKeyGroupName = key.Group.Name
	}
	return nil
}

func (s *ModelRadarService) loadExistingAPIKey(ctx context.Context, id int64) (*APIKey, error) {
	if s == nil || s.apiKeyReader == nil {
		return nil, ErrModelRadarAPIKeyInvalid
	}
	key, err := s.apiKeyReader.GetByID(ctx, id)
	if err != nil {
		return nil, ErrModelRadarAPIKeyInvalid
	}
	return key, nil
}

func validateModelRadarExistingAPIKey(key *APIKey) error {
	if key == nil || strings.TrimSpace(key.Key) == "" {
		return ErrModelRadarAPIKeyInvalid
	}
	if key.Status != StatusActive {
		return ErrModelRadarAPIKeyInvalid
	}
	if key.IsExpired() || key.IsQuotaExhausted() {
		return ErrModelRadarAPIKeyInvalid
	}
	return nil
}

func defaultModelRadarConfig() ModelRadarConfig {
	return ModelRadarConfig{
		APIKeySource:        ModelRadarAPIKeySourceCustom,
		RunHour:             modelRadarDefaultRunHour,
		RunMinute:           modelRadarDefaultRunMinute,
		TimeoutSeconds:      modelRadarDefaultTimeoutSec,
		Concurrency:         modelRadarDefaultConcurrency,
		DailyBudgetUSDCents: modelRadarDefaultBudgetCents,
		Matrix:              defaultModelRadarMatrix(),
	}
}

func defaultModelRadarMatrix() []ModelRadarCombination {
	return []ModelRadarCombination{
		{Model: "gpt-5.5", ReasoningEffort: "xhigh", Enabled: true},
		{Model: "gpt-5.5", ReasoningEffort: "high", Enabled: true},
		{Model: "gpt-5.5", ReasoningEffort: "medium", Enabled: true},
		{Model: "gpt-5.4", ReasoningEffort: "xhigh", Enabled: true},
		{Model: "gpt-5.4", ReasoningEffort: "high", Enabled: true},
		{Model: "gpt-5.4", ReasoningEffort: "medium", Enabled: true},
	}
}

func normalizeModelRadarConfig(cfg ModelRadarConfig) ModelRadarConfig {
	cfg.APIBaseURL = normalizeModelRadarGatewayBaseURL(cfg.APIBaseURL)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.APIKeySource = strings.TrimSpace(cfg.APIKeySource)
	if cfg.APIKeySource == "" {
		cfg.APIKeySource = ModelRadarAPIKeySourceCustom
	}
	if cfg.APIKeySource != ModelRadarAPIKeySourceExisting {
		cfg.APIKeySource = ModelRadarAPIKeySourceCustom
	}
	if cfg.RunHour < 0 || cfg.RunHour > 23 {
		cfg.RunHour = modelRadarDefaultRunHour
	}
	if cfg.RunMinute < 0 || cfg.RunMinute > 59 {
		cfg.RunMinute = modelRadarDefaultRunMinute
	}
	if cfg.TimeoutSeconds < 10 || cfg.TimeoutSeconds > 300 {
		cfg.TimeoutSeconds = modelRadarDefaultTimeoutSec
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 6 {
		cfg.Concurrency = modelRadarDefaultConcurrency
	}
	if cfg.DailyBudgetUSDCents < 0 {
		cfg.DailyBudgetUSDCents = modelRadarDefaultBudgetCents
	}
	if len(cfg.Matrix) == 0 {
		cfg.Matrix = defaultModelRadarMatrix()
	}
	out := make([]ModelRadarCombination, 0, len(cfg.Matrix))
	seen := map[string]struct{}{}
	for _, combo := range cfg.Matrix {
		combo.Model = strings.TrimSpace(combo.Model)
		combo.ReasoningEffort = strings.TrimSpace(combo.ReasoningEffort)
		if combo.Model == "" {
			continue
		}
		key := combo.Model + "\x00" + combo.ReasoningEffort
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, combo)
	}
	cfg.Matrix = out
	return cfg
}

func normalizeModelRadarGatewayBaseURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(base, "/chat/completions"):
		return normalizeModelRadarGatewayBaseURL(strings.TrimSuffix(base, "/chat/completions"))
	case strings.HasSuffix(base, "/api/v1"):
		return strings.TrimSuffix(base, "/api/v1") + "/v1"
	case strings.HasSuffix(base, "/v1"):
		return base
	default:
		return base + "/v1"
	}
}

func enabledModelRadarMatrix(matrix []ModelRadarCombination) []ModelRadarCombination {
	out := make([]ModelRadarCombination, 0, len(matrix))
	for _, combo := range matrix {
		if combo.Enabled {
			out = append(out, combo)
		}
	}
	return out
}

func sortModelRadarResults(results []*ModelRadarResult) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.PassCount != b.PassCount {
			return a.PassCount > b.PassCount
		}
		if a.AvgLatencyMs != nil && b.AvgLatencyMs != nil && *a.AvgLatencyMs != *b.AvgLatencyMs {
			return *a.AvgLatencyMs < *b.AvgLatencyMs
		}
		if a.Model != b.Model {
			return a.Model > b.Model
		}
		return a.ReasoningEffort < b.ReasoningEffort
	})
}

func sortModelRadarTaskCarriers(items []modelRadarTaskCarrier) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Result, items[j].Result
		if a == nil || b == nil {
			return b == nil
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.PassCount != b.PassCount {
			return a.PassCount > b.PassCount
		}
		if a.AvgLatencyMs != nil && b.AvgLatencyMs != nil && *a.AvgLatencyMs != *b.AvgLatencyMs {
			return *a.AvgLatencyMs < *b.AvgLatencyMs
		}
		if a.Model != b.Model {
			return a.Model > b.Model
		}
		return a.ReasoningEffort < b.ReasoningEffort
	})
}

func calculateModelRadarScore(tasks []*ModelRadarTaskResult, avgLatencyMs *int) int {
	if len(tasks) == 0 {
		return 0
	}
	weights := map[string]int{}
	totalWeight := 0
	for _, task := range modelRadarTaskSet() {
		weight := task.Weight
		if weight <= 0 {
			weight = 1
		}
		weights[task.ID] = weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	passedWeight := 0
	for _, task := range tasks {
		if task == nil || !task.Passed {
			continue
		}
		weight := weights[task.TaskID]
		if weight <= 0 {
			weight = 1
		}
		passedWeight += weight
	}
	qualityScore := float64(passedWeight) / float64(totalWeight) * 92
	speedScore := 0.0
	if avgLatencyMs != nil && *avgLatencyMs > 0 {
		switch {
		case *avgLatencyMs <= 2500:
			speedScore = 8
		case *avgLatencyMs >= 9000:
			speedScore = 0
		default:
			speedScore = (9000 - float64(*avgLatencyMs)) / 6500 * 8
		}
	}
	score := int(math.Round(qualityScore + speedScore))
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func modelRadarLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*3600)
	}
	return loc
}

func maskModelRadarKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 10 {
		return "已配置"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func truncateModelRadarAnswer(answer string) string {
	answer = strings.TrimSpace(answer)
	if len(answer) > 512 {
		return answer[:512]
	}
	return answer
}

func sanitizeModelRadarError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	msg = modelRadarSecretPattern.ReplaceAllString(msg, "[redacted-key]")
	if len(msg) > 600 {
		msg = msg[:600]
	}
	return msg
}
