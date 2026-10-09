package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const telegramTestToken = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghij"

type telegramTestSettingsRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
	err    error
}

func (r *telegramTestSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return "", r.err
	}
	if raw, ok := r.values[key]; ok {
		return raw, nil
	}
	return "", ErrSettingNotFound
}

func (r *telegramTestSettingsRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.values[key] = value
	return nil
}

type telegramTestCipher struct{ err error }

func (e telegramTestCipher) Encrypt(raw string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	return "enc:" + base64.StdEncoding.EncodeToString([]byte(raw)), nil
}

func (e telegramTestCipher) Decrypt(raw string) (string, error) {
	if e.err != nil {
		return "", e.err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, "enc:"))
	return string(decoded), err
}

type telegramTestHTTPClient func(*http.Request) (*http.Response, error)

func (f telegramTestHTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

type telegramTestTransport func(*http.Request) (*http.Response, error)

func (f telegramTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTelegramTestService(t *testing.T) (*TelegramNotificationService, *telegramTestSettingsRepo) {
	t.Helper()
	repo := &telegramTestSettingsRepo{values: map[string]string{}}
	return NewTelegramNotificationService(repo, telegramTestCipher{}, nil), repo
}

func TestTelegramSettingsTokenEncryptedAndNeverReturned(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTelegramTestService(t)
	settings, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.False(t, settings.BotTokenConfigured)
	settings, err = svc.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.True(t, settings.BotTokenConfigured)
	require.NotContains(t, repo.values[SettingKeyTelegramPaymentNotifications], telegramTestToken)
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NotContains(t, string(raw), telegramTestToken)
	require.NotContains(t, string(raw), "encrypted_bot_token")
	var keys map[string]any
	require.NoError(t, json.Unmarshal(raw, &keys))
	require.Len(t, keys, 4)

	before := repo.values[SettingKeyTelegramPaymentNotifications]
	settings, err = svc.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, ChatID: "-1001234567891"})
	require.NoError(t, err)
	require.True(t, settings.BotTokenConfigured)
	require.NotEqual(t, before, repo.values[SettingKeyTelegramPaymentNotifications])
	cfg, err := svc.loadSettings(ctx)
	require.NoError(t, err)
	token, err := svc.botToken(cfg)
	require.NoError(t, err)
	require.Equal(t, telegramTestToken, token)

	settings, err = svc.UpdateSettings(ctx, UpdateTelegramSettingsRequest{ClearBotToken: true})
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.False(t, settings.BotTokenConfigured)
}

func TestTelegramSettingsValidationDoesNotPartiallySave(t *testing.T) {
	cases := []UpdateTelegramSettingsRequest{
		{Enabled: true},
		{Enabled: true, ChatID: "-1001234567890"},
		{BotToken: "bad-secret-token", ChatID: "-1001234567890"},
		{BotToken: telegramTestToken, ChatID: "@username"},
		{BotToken: telegramTestToken, ChatID: "123456"},
		{BotToken: telegramTestToken, ChatID: "-0"},
		{BotToken: telegramTestToken, ChatID: "-9223372036854775809"},
		{BotToken: telegramTestToken, ChatID: "-100123/../../other"},
		{BotToken: telegramTestToken, ClearBotToken: true},
	}
	for _, req := range cases {
		t.Run(req.ChatID+req.BotToken, func(t *testing.T) {
			svc, repo := newTelegramTestService(t)
			_, err := svc.UpdateSettings(context.Background(), req)
			require.Error(t, err)
			require.Empty(t, repo.values)
			require.NotContains(t, err.Error(), telegramTestToken)
			require.NotContains(t, err.Error(), "bad-secret-token")
		})
	}
}

func TestTelegramOrderVerificationCanBeConfiguredWithoutBot(t *testing.T) {
	svc, repo := newTelegramTestService(t)
	settings, err := svc.UpdateSettings(context.Background(), UpdateTelegramSettingsRequest{VerifyEasyPayOrders: true})
	require.NoError(t, err)
	require.True(t, settings.VerifyEasyPayOrders)
	require.False(t, settings.Enabled)
	require.False(t, settings.BotTokenConfigured)
	stored, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	require.True(t, stored.VerifyEasyPayOrders)
	require.NotEmpty(t, repo.values[SettingKeyTelegramPaymentNotifications])
}

func TestTelegramSettingsErrorsDoNotExposeSecrets(t *testing.T) {
	svc, repo := newTelegramTestService(t)
	svc.encryptor = telegramTestCipher{err: errors.New("failed to encrypt " + telegramTestToken)}
	_, err := svc.UpdateSettings(context.Background(), UpdateTelegramSettingsRequest{BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), telegramTestToken)
	require.Empty(t, repo.values)
	repo.values[SettingKeyTelegramPaymentNotifications] = "invalid json with " + telegramTestToken
	_, err = svc.GetSettings(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), telegramTestToken)
}

func TestTelegramTestMessageUsesDraftWithoutSaving(t *testing.T) {
	svc, repo := newTelegramTestService(t)
	_, err := svc.UpdateSettings(context.Background(), UpdateTelegramSettingsRequest{BotToken: telegramTestToken, ChatID: "-1001234567890"})
	require.NoError(t, err)
	before := repo.values[SettingKeyTelegramPaymentNotifications]
	var sentChatIDs []string
	svc.httpClient = telegramTestHTTPClient(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https", req.URL.Scheme)
		require.Equal(t, "api.telegram.org", req.URL.Host)
		require.Equal(t, "/bot"+telegramTestToken+"/sendMessage", req.URL.Path)
		require.Equal(t, http.MethodPost, req.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.NotContains(t, body, "parse_mode")
		sentChatIDs = append(sentChatIDs, body["chat_id"].(string))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})
	require.NoError(t, svc.Test(context.Background(), UpdateTelegramSettingsRequest{BotToken: telegramTestToken, ChatID: "-1009999999999"}))
	require.NoError(t, svc.Test(context.Background(), UpdateTelegramSettingsRequest{}))
	require.Equal(t, []string{"-1009999999999", "-1001234567890"}, sentChatIDs)
	require.Equal(t, before, repo.values[SettingKeyTelegramPaymentNotifications])
}

func TestTelegramHTTPFailuresSanitizedAndClassified(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		transport  bool
		permanent  bool
		retryAfter time.Duration
	}{
		{name: "network", transport: true},
		{name: "bad token", status: 401, body: `{"ok":false,"description":"` + telegramTestToken + `"}`},
		{name: "forbidden", status: 403, body: `{"ok":false,"description":"private user data"}`},
		{name: "bot not found", status: 404, body: `{"ok":false,"description":"not found"}`},
		{name: "invalid chat", status: 200, body: `{"ok":false,"error_code":400,"description":"raw callback secret"}`},
		{name: "rate limited", status: 429, body: `{"ok":false,"parameters":{"retry_after":77}}`, retryAfter: 77 * time.Second},
		{name: "server error", status: 502, body: "upstream secret data"},
		{name: "invalid json", status: 200, body: "not json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTelegramTestService(t)
			svc.httpClient = telegramTestHTTPClient(func(req *http.Request) (*http.Response, error) {
				if tc.transport {
					return nil, errors.New("Post " + req.URL.String() + ": secret transport failure")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			err := svc.sendMessage(context.Background(), telegramTestToken, "-1001234567890", "test")
			require.Error(t, err)
			var failed *telegramSendError
			require.ErrorAs(t, err, &failed)
			require.Equal(t, tc.permanent, failed.permanent)
			require.Equal(t, tc.retryAfter, failed.retryAfter)
			require.NotContains(t, err.Error(), telegramTestToken)
			require.NotContains(t, err.Error(), "secret")
			require.NotContains(t, err.Error(), "private user data")
		})
	}
}

func TestTelegramHTTPRedirectCannotLeakToken(t *testing.T) {
	svc, _ := newTelegramTestService(t)
	client := svc.httpClient.(*http.Client)
	requests := 0
	client.Transport = telegramTestTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		require.Equal(t, "api.telegram.org", req.URL.Host)
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://attacker.invalid/capture"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	})
	err := svc.sendMessage(context.Background(), telegramTestToken, "-1001234567890", "test")
	require.Error(t, err)
	require.Equal(t, 1, requests)
	require.NotContains(t, err.Error(), telegramTestToken)
}

func TestTelegramPaymentMessageSafePlainText(t *testing.T) {
	event := TelegramPaymentEvent{EventKey: "payment:9:completed", OrderID: 9, UserID: 2, Username: "User\n状态：伪造", Email: "user@example.test", OccurredAt: time.Date(2026, 10, 7, 1, 2, 3, 0, time.UTC), PayAmount: 7.2, Currency: "CNY", CreditedAmount: 1, BonusAmount: 0.2, OutTradeNo: "order-9", PaymentType: "alipay", Outcome: TelegramOutcomeRechargeSuccess, Reason: "签名、订单金额及归属校验通过"}
	message := formatTelegramPaymentEvent(event)
	require.Contains(t, message, "时间：2026-10-07 09:02:03（北京时间）")
	require.Contains(t, message, "用户名：User 状态：伪造")
	require.Contains(t, message, "实付：7.20 CNY")
	require.Contains(t, message, "状态：已核验并入账")
	require.Less(t, len([]rune(message)), 4096)
	event.Outcome = TelegramOutcomePaymentReview
	require.Contains(t, formatTelegramPaymentEvent(event), "异常回调已拦截，待复核")
	require.Contains(t, formatTelegramPaymentEvent(event), "应付：7.20 CNY")
	require.NotContains(t, formatTelegramPaymentEvent(event), "入账：")
}
