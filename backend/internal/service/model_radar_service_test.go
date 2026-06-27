package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelRadarEvaluateAnswer_NormalizesExactShortAnswers(t *testing.T) {
	task := modelRadarTask{Expected: "alpha,bravo,charlie"}
	require.True(t, evaluateModelRadarAnswer(task, "答案是：\n`alpha, bravo, charlie`"))
	require.False(t, evaluateModelRadarAnswer(task, "alpha,charlie,bravo"))
}

func TestModelRadarEvaluateAnswer_NormalizesJSONAnswers(t *testing.T) {
	task := modelRadarTask{Expected: `{"numbers":[2,5,8],"sum":15}`}

	require.True(t, evaluateModelRadarAnswer(task, "{\n  \"sum\": 15,\n  \"numbers\": [2, 5, 8]\n}"))
	require.False(t, evaluateModelRadarAnswer(task, `{"numbers":[2,5,8],"sum":16}`))
}

func TestSortModelRadarResultsStableByScorePassLatencyModel(t *testing.T) {
	latFast := 100
	latSlow := 200
	items := []*ModelRadarResult{
		{Model: "gpt-5.4", ReasoningEffort: "high", Score: 75, PassCount: 9, AvgLatencyMs: &latSlow},
		{Model: "gpt-5.5", ReasoningEffort: "medium", Score: 75, PassCount: 9, AvgLatencyMs: &latFast},
		{Model: "gpt-5.4", ReasoningEffort: "medium", Score: 90, PassCount: 11, AvgLatencyMs: &latSlow},
	}
	sortModelRadarResults(items)
	require.Equal(t, "gpt-5.4", items[0].Model)
	require.Equal(t, "medium", items[0].ReasoningEffort)
	require.Equal(t, "gpt-5.5", items[1].Model)
}

func TestCalculateModelRadarScoreUsesTaskWeightAndLatency(t *testing.T) {
	tasks := modelRadarTaskSet()
	allPassed := make([]*ModelRadarTaskResult, 0, len(tasks))
	missedWeighted := make([]*ModelRadarTaskResult, 0, len(tasks))
	for _, task := range tasks {
		allPassed = append(allPassed, &ModelRadarTaskResult{TaskID: task.ID, Passed: true})
		missedWeighted = append(missedWeighted, &ModelRadarTaskResult{
			TaskID: task.ID,
			Passed: task.ID != "instruction-01",
		})
	}

	fast := 2200
	slow := 8500

	require.Equal(t, 100, calculateModelRadarScore(allPassed, &fast))
	require.Less(t, calculateModelRadarScore(allPassed, &slow), 100)
	require.Less(t, calculateModelRadarScore(missedWeighted, &fast), calculateModelRadarScore(allPassed, &slow))
}

func TestModelRadarConfigMasksAPIKey(t *testing.T) {
	repo := &modelRadarRepoStub{}
	settingRepo := newModelRadarSettingRepoStub(map[string]string{
		SettingKeyModelRadarEnabled: "true",
		SettingKeyAPIBaseURL:        "https://api.example.com",
	})
	svc := NewModelRadarService(repo, settingRepo, nil, plainModelRadarEncryptor{}, nil)

	cfg, err := svc.UpdateConfig(context.Background(), ModelRadarConfig{
		Enabled:        true,
		APIBaseURL:     "https://api.example.com",
		APIKeySource:   ModelRadarAPIKeySourceCustom,
		APIKey:         "sk-test-secret-value",
		RunHour:        4,
		RunMinute:      30,
		TimeoutSeconds: 90,
		Concurrency:    2,
		Matrix:         defaultModelRadarMatrix(),
	})
	require.NoError(t, err)
	require.True(t, cfg.APIKeyConfigured)
	require.Empty(t, cfg.APIKey)
	require.Equal(t, ModelRadarAPIKeySourceCustom, cfg.APIKeySource)
	require.Equal(t, "sk-tes...alue", cfg.APIKeyMasked)
	require.NotContains(t, settingRepo.values[SettingKeyModelRadarConfig], "sk-test-secret-value")
}

func TestModelRadarConfigExistingAPIKeyStoresReferenceWithoutPlainKey(t *testing.T) {
	repo := &modelRadarRepoStub{}
	apiKeys := &modelRadarAPIKeyServiceStub{
		keys: map[int64]*APIKey{
			7: {
				ID:      7,
				UserID:  42,
				Key:     "sk-existing-secret-value",
				Name:    "radar-key",
				Status:  StatusActive,
				GroupID: modelRadarPtrInt64(3),
				Group:   &Group{ID: 3, Name: "pro"},
			},
		},
	}
	settingRepo := newModelRadarSettingRepoStub(map[string]string{
		SettingKeyModelRadarEnabled: "true",
		SettingKeyAPIBaseURL:        "https://api.example.com",
	})
	svc := NewModelRadarService(repo, settingRepo, nil, plainModelRadarEncryptor{}, apiKeys)

	cfg, err := svc.UpdateConfigWithOptions(context.Background(), ModelRadarConfig{
		Enabled:        true,
		APIBaseURL:     "https://api.example.com",
		APIKeySource:   ModelRadarAPIKeySourceExisting,
		APIKeyID:       modelRadarPtrInt64(7),
		APIKey:         "sk-should-not-be-saved",
		RunHour:        4,
		RunMinute:      30,
		TimeoutSeconds: 90,
		Concurrency:    2,
		Matrix:         defaultModelRadarMatrix(),
	}, ModelRadarConfigUpdateOptions{AdminUserID: 42})

	require.NoError(t, err)
	require.True(t, cfg.APIKeyConfigured)
	require.Empty(t, cfg.APIKey)
	require.Equal(t, ModelRadarAPIKeySourceExisting, cfg.APIKeySource)
	require.Equal(t, int64(7), *cfg.APIKeyID)
	require.Equal(t, "radar-key", cfg.APIKeyName)
	require.Equal(t, "pro", cfg.APIKeyGroupName)
	require.Equal(t, StatusActive, cfg.APIKeyStatus)
	require.NotContains(t, settingRepo.values[SettingKeyModelRadarConfig], "sk-existing-secret-value")
	require.NotContains(t, settingRepo.values[SettingKeyModelRadarConfig], "sk-should-not-be-saved")
}

func TestModelRadarConfigExistingAPIKeyRejectsOtherOwner(t *testing.T) {
	repo := &modelRadarRepoStub{}
	apiKeys := &modelRadarAPIKeyServiceStub{
		keys: map[int64]*APIKey{
			7: {ID: 7, UserID: 99, Key: "sk-existing-secret-value", Name: "other", Status: StatusActive},
		},
	}
	settingRepo := newModelRadarSettingRepoStub(map[string]string{SettingKeyModelRadarEnabled: "true"})
	svc := NewModelRadarService(repo, settingRepo, nil, plainModelRadarEncryptor{}, apiKeys)

	_, err := svc.UpdateConfigWithOptions(context.Background(), ModelRadarConfig{
		Enabled:      true,
		APIKeySource: ModelRadarAPIKeySourceExisting,
		APIKeyID:     modelRadarPtrInt64(7),
		Matrix:       defaultModelRadarMatrix(),
	}, ModelRadarConfigUpdateOptions{AdminUserID: 42})

	require.Error(t, err)
}

func TestModelRadarScheduledRunReusesExistingRunForSameDay(t *testing.T) {
	runDate := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	repo := &modelRadarRepoStub{
		existingRun: &ModelRadarRun{
			ID:          42,
			RunDate:     runDate,
			TriggerType: ModelRadarTriggerScheduled,
			Status:      ModelRadarStatusFailed,
			CreatedAt:   runDate,
			UpdatedAt:   runDate,
		},
	}
	settingRepo := newModelRadarSettingRepoStub(map[string]string{
		SettingKeyModelRadarEnabled: "true",
		SettingKeyModelRadarConfig:  `{"enabled":true,"api_base_url":"https://api.example.com","api_key":"encrypted-secret","run_hour":4,"run_minute":30,"timeout_seconds":90,"concurrency":2,"matrix":[{"model":"gpt-5.5","reasoning_effort":"medium","enabled":true}]}`,
	})
	svc := NewModelRadarService(repo, settingRepo, nil, plainModelRadarEncryptor{}, nil)

	detail, err := svc.Run(context.Background(), ModelRadarRunParams{
		TriggerType: ModelRadarTriggerScheduled,
		Now:         time.Date(2026, 6, 27, 8, 0, 0, 0, modelRadarLocation()),
	})

	require.NoError(t, err)
	require.Equal(t, int64(42), detail.Run.ID)
	require.Equal(t, 0, repo.createdRuns)
}

func TestModelRadarRunNowReturnsRunningRunWithoutWaitingForMatrix(t *testing.T) {
	repo := &modelRadarRepoStub{}
	settingRepo := newModelRadarSettingRepoStub(map[string]string{
		SettingKeyModelRadarEnabled: "true",
		SettingKeyModelRadarConfig:  `{"enabled":true,"api_base_url":"https://api.example.com","api_key":"encrypted-secret","run_hour":4,"run_minute":30,"timeout_seconds":10,"concurrency":1,"matrix":[{"model":"gpt-5.5","reasoning_effort":"medium","enabled":true}]}`,
	})
	svc := NewModelRadarService(repo, settingRepo, nil, plainModelRadarEncryptor{}, nil)
	blockingClient := &blockingModelRadarHTTPClient{started: make(chan struct{}), release: make(chan struct{})}
	svc.SetHTTPClient(blockingClient)

	ctx, cancel := context.WithCancel(context.Background())
	detail, err := svc.RunNow(ctx)
	cancel()

	require.NoError(t, err)
	require.NotNil(t, detail.Run)
	require.Equal(t, ModelRadarStatusRunning, detail.Run.Status)
	require.Equal(t, ModelRadarTriggerManual, detail.Run.TriggerType)
	require.Empty(t, detail.Results)
	require.Equal(t, 1, repo.createdRuns)
	require.Eventually(t, func() bool {
		select {
		case <-blockingClient.started:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	close(blockingClient.release)
	require.Eventually(t, func() bool { return repo.updatedRuns > 0 }, time.Second, 10*time.Millisecond)
}

func TestSanitizeModelRadarErrorRedactsAPIKeys(t *testing.T) {
	msg := sanitizeModelRadarError(errors.New("upstream rejected key sk-test-secret-value-1234567890"))

	require.Contains(t, msg, "[redacted-key]")
	require.NotContains(t, msg, "sk-test-secret-value")
}

func TestNormalizeModelRadarGatewayBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com":                         "https://example.com/v1",
		"https://example.com/":                        "https://example.com/v1",
		"https://example.com/v1":                      "https://example.com/v1",
		"https://example.com/api/v1":                  "https://example.com/v1",
		"https://example.com/v1/chat/completions":     "https://example.com/v1",
		"https://example.com/api/v1/chat/completions": "https://example.com/v1",
	}
	for input, want := range cases {
		require.Equal(t, want, normalizeModelRadarGatewayBaseURL(input), input)
	}
}

type plainModelRadarEncryptor struct{}

func (plainModelRadarEncryptor) Encrypt(string) (string, error) { return "encrypted-secret", nil }
func (plainModelRadarEncryptor) Decrypt(string) (string, error) { return "sk-test-secret-value", nil }

type modelRadarAPIKeyServiceStub struct {
	keys map[int64]*APIKey
}

func (s *modelRadarAPIKeyServiceStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	if key, ok := s.keys[id]; ok {
		copy := *key
		return &copy, nil
	}
	return nil, ErrAPIKeyNotFound
}

func modelRadarPtrInt64(v int64) *int64 { return &v }

type blockingModelRadarHTTPClient struct {
	started chan struct{}
	release chan struct{}
}

func (c *blockingModelRadarHTTPClient) Do(*http.Request) (*http.Response, error) {
	select {
	case <-c.started:
	default:
		close(c.started)
	}
	<-c.release
	return nil, errors.New("released")
}

type modelRadarSettingRepoStub struct {
	values map[string]string
}

func newModelRadarSettingRepoStub(values map[string]string) *modelRadarSettingRepoStub {
	return &modelRadarSettingRepoStub{values: values}
}

func (r *modelRadarSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *modelRadarSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (r *modelRadarSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
func (r *modelRadarSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}
func (r *modelRadarSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}
func (r *modelRadarSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}
func (r *modelRadarSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

type modelRadarRepoStub struct {
	existingRun *ModelRadarRun
	createdRuns int
	updatedRuns int
}

func (r *modelRadarRepoStub) CreateRun(_ context.Context, run *ModelRadarRun) error {
	r.createdRuns++
	run.ID = int64(r.createdRuns)
	return nil
}
func (r *modelRadarRepoStub) GetScheduledRunByDate(context.Context, time.Time) (*ModelRadarRun, error) {
	if r.existingRun != nil {
		return r.existingRun, nil
	}
	return nil, ErrModelRadarRunNotFound
}
func (r *modelRadarRepoStub) UpdateRun(context.Context, *ModelRadarRun) error {
	r.updatedRuns++
	return nil
}
func (*modelRadarRepoStub) InsertResult(context.Context, *ModelRadarResult, []*ModelRadarTaskResult) error {
	return nil
}
func (*modelRadarRepoStub) ListRuns(context.Context, int) ([]*ModelRadarRun, error) { return nil, nil }
func (r *modelRadarRepoStub) GetRun(_ context.Context, id int64) (*ModelRadarRun, error) {
	if r.existingRun != nil && r.existingRun.ID == id {
		return r.existingRun, nil
	}
	return nil, ErrModelRadarRunNotFound
}
func (*modelRadarRepoStub) ListResults(context.Context, int64) ([]*ModelRadarResult, error) {
	return nil, nil
}
func (*modelRadarRepoStub) ListTaskResults(context.Context, []int64) (map[int64][]*ModelRadarTaskResult, error) {
	return map[int64][]*ModelRadarTaskResult{}, nil
}
func (*modelRadarRepoStub) GetLatestPublishedRun(context.Context) (*ModelRadarRun, error) {
	return nil, ErrModelRadarRunNotFound
}
func (*modelRadarRepoStub) ListPublishedBestResults(context.Context, int) ([]*ModelRadarResult, error) {
	return nil, nil
}
