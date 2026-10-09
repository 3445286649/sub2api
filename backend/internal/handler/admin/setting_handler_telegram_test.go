package admin

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type telegramHandlerSettingsRepo struct {
	service.SettingRepository
	value string
}

func (r *telegramHandlerSettingsRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}

func (r *telegramHandlerSettingsRepo) Set(_ context.Context, _ string, value string) error {
	r.value = value
	return nil
}

type telegramHandlerCipher struct{}

func (telegramHandlerCipher) Encrypt(value string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(value)), nil
}

func (telegramHandlerCipher) Decrypt(value string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	return string(raw), err
}

func TestTelegramSettingsHandlersPreserveSecretAndValidateInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &telegramHandlerSettingsRepo{}
	h := &SettingHandler{telegramService: service.NewTelegramNotificationService(repo, telegramHandlerCipher{}, nil)}
	router := gin.New()
	router.GET("/telegram", h.GetTelegramSettings)
	router.PUT("/telegram", h.UpdateTelegramSettings)
	router.POST("/telegram/test", h.TestTelegramSettings)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	const token = "123456789:ABCDEFGHIJKLMNOPQRSTUV_abcdefghijk"
	w := request(http.MethodPut, "/telegram", `{"enabled":true,"verify_easypay_orders":true,"bot_token":"`+token+`","chat_id":"-1001234567890"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), token)
	require.NotContains(t, repo.value, token)
	require.Contains(t, w.Body.String(), `"bot_token_configured":true`)
	require.Contains(t, w.Body.String(), `"verify_easypay_orders":true`)
	w = request(http.MethodGet, "/telegram", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), token)
	require.NotContains(t, w.Body.String(), "encrypted_bot_token")
	before := repo.value
	for _, body := range []string{
		`{}`,
		`{"enabled":true}`,
		`{"enabled":null,"verify_easypay_orders":true}`,
		`{"enabled":true,"chat_id":"100"}`,
		`{"bot_token":"bad/token"}`,
		`{"bot_token":123}`,
		`{"bot_token":"` + token + strings.Repeat("x", 4096) + `"}`,
	} {
		w = request(http.MethodPut, "/telegram", body)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.NotContains(t, w.Body.String(), token)
		require.Equal(t, before, repo.value)
	}
	w = request(http.MethodPost, "/telegram/test", `{"bot_token":"invalid","chat_id":"-1"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, before, repo.value, "test settings must not save the draft")
	w = request(http.MethodPut, "/telegram", `{"enabled":false,"verify_easypay_orders":true,"clear_bot_token":true}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"bot_token_configured":false`)
	require.Contains(t, w.Body.String(), `"verify_easypay_orders":true`)
}

func TestTelegramSettingsHandlersUnavailableService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SettingHandler{}
	for _, handler := range []gin.HandlerFunc{h.GetTelegramSettings, h.UpdateTelegramSettings, h.TestTelegramSettings} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/telegram", nil)
		handler(c)
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
	}
}
