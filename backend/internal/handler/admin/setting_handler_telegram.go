package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetTelegramSettings(c *gin.Context) {
	if h.telegramService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Telegram service is unavailable")
		return
	}
	settings, err := h.telegramService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) UpdateTelegramSettings(c *gin.Context) {
	req, ok := h.bindTelegramSettings(c)
	if !ok {
		return
	}
	settings, err := h.telegramService.UpdateSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) TestTelegramSettings(c *gin.Context) {
	req, ok := h.bindTelegramSettings(c)
	if !ok {
		return
	}
	if err := h.telegramService.Test(c.Request.Context(), req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Telegram test message sent"})
}

func (h *SettingHandler) bindTelegramSettings(c *gin.Context) (service.UpdateTelegramSettingsRequest, bool) {
	var req service.UpdateTelegramSettingsRequest
	if h.telegramService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Telegram service is unavailable")
		return req, false
	}
	var input struct {
		Enabled             *bool  `json:"enabled"`
		VerifyEasyPayOrders *bool  `json:"verify_easypay_orders"`
		BotToken            string `json:"bot_token"`
		ChatID              string `json:"chat_id"`
		ClearBotToken       bool   `json:"clear_bot_token"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if err := c.ShouldBindJSON(&input); err != nil || input.Enabled == nil || input.VerifyEasyPayOrders == nil {
		response.BadRequest(c, "Invalid Telegram settings request")
		return req, false
	}
	req.Enabled, req.VerifyEasyPayOrders = *input.Enabled, *input.VerifyEasyPayOrders
	req.BotToken, req.ChatID, req.ClearBotToken = input.BotToken, input.ChatID, input.ClearBotToken
	return req, true
}
