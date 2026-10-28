package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) DeepSeekSearch(c *gin.Context) {
	h.deepSeekTool(c, service.DeepSeekToolSearch)
}

func (h *OpenAIGatewayHandler) DeepSeekFetch(c *gin.Context) {
	h.deepSeekTool(c, service.DeepSeekToolFetch)
}

func (h *OpenAIGatewayHandler) deepSeekTool(c *gin.Context, tool service.DeepSeekTool) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	setOpenAIClientTransportHTTP(c)
	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group.Platform != service.PlatformDeepseek {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "DeepSeek tools are only available for DeepSeek groups")
		return
	}
	if !deepSeekGroupToolEnabled(apiKey.Group, tool) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "This DeepSeek tool is disabled for the API key group")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if !h.ensureResponsesDependencies(c, requestLogger(c, "handler.deepseek_tools")) {
		return
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if err := validateDeepSeekToolRequest(tool, body); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	toolModel := "deepseek-tool-" + string(tool)
	reqLog := requestLogger(c, "handler.deepseek_tools",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("tool", string(tool)),
	)
	setOpsRequestContext(c, toolModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, "deepseek_tools", toolModel, body); decision != nil && !decision.AllowNextStage {
		h.openAISecurityAuditError(c, decision)
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, apiKey.ID, apiKey.ConcurrencyLimit, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}
	inflightDone, inflightErr := reserveInflightBalance(c, h.billingCacheService, h.gatewayService, apiKey, subscription, service.InflightEstimateRequest{
		Model:          toolModel,
		Kind:           service.InflightEstimatePerRequest,
		WebSearchCalls: 1,
	})
	if inflightErr != nil {
		status, code, message, retryAfter := billingErrorDetails(inflightErr)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}
	defer inflightDone()

	sessionHash := h.gatewayService.GenerateSessionHashWithFallback(c, nil, service.ExtractClientSessionID(c))
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError
	maxSwitches := h.maxAccountSwitches
	if maxSwitches < 0 {
		maxSwitches = 0
	}
	for switchCount := 0; ; {
		selection, _, selectErr := h.gatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(), apiKey.GroupID, "", sessionHash, "", failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilityChatCompletions,
			false, false, false, service.PlatformDeepseek,
		)
		if selectErr != nil || selection == nil || selection.Account == nil {
			if errors.Is(c.Request.Context().Err(), context.Canceled) {
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
			} else {
				markOpsRoutingCapacityLimitedIfNoAvailable(c, selectErr)
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available DeepSeek accounts")
			}
			return
		}

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		accountRelease, slotResult := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
		if slotResult == openAISlotAcquireProfitVetoed {
			failedAccountIDs[account.ID] = struct{}{}
			if switchCount >= maxSwitches {
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available DeepSeek accounts")
				return
			}
			switchCount++
			continue
		}
		if slotResult != openAISlotAcquireOK {
			return
		}

		forwardStart := time.Now()
		result, forwardErr := h.gatewayService.ForwardDeepSeekTool(c.Request.Context(), c, account, tool, body)
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, time.Since(forwardStart).Milliseconds())
		if accountRelease != nil {
			accountRelease()
		}
		if forwardErr == nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account, toolModel, true, nil)
			if result != nil && result.WebSearchCalls > 0 {
				h.recordDeepSeekToolUsage(c, apiKey, account, subscription, toolModel, body, result, subject.UserID)
			}
			return
		}
		if c.Request.Context().Err() != nil {
			return
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account, toolModel, false, nil, forwardErr)
		failedAccountIDs[account.ID] = struct{}{}
		lastFailoverErr = nil
		if !errors.As(forwardErr, &lastFailoverErr) {
			lastFailoverErr = &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway}
		}
		if switchCount >= maxSwitches {
			h.handleFailoverExhausted(c, lastFailoverErr, false)
			return
		}
		switchCount++
	}
}

func deepSeekGroupToolEnabled(group *service.Group, tool service.DeepSeekTool) bool {
	if group == nil || group.Platform != service.PlatformDeepseek {
		return false
	}
	switch tool {
	case service.DeepSeekToolSearch:
		return group.AllowDeepSeekSearch
	case service.DeepSeekToolFetch:
		return group.AllowDeepSeekFetch
	default:
		return false
	}
}

func validateDeepSeekToolRequest(tool service.DeepSeekTool, body []byte) error {
	if len(body) > 1<<20 || !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return errors.New("Request body must be a JSON object no larger than 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if _, err := decoder.Token(); err != nil {
		return errors.New("Invalid JSON object")
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return errors.New("Invalid JSON object")
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("Invalid JSON object")
		}
		if _, exists := seen[key]; exists {
			return errors.New("Duplicate JSON field: " + key)
		}
		seen[key] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return errors.New("Invalid JSON object")
		}
	}
	switch tool {
	case service.DeepSeekToolSearch:
		query := gjson.GetBytes(body, "query")
		if query.Type != gjson.String || utf8.RuneCountInString(strings.TrimSpace(query.String())) < 1 || utf8.RuneCountInString(strings.TrimSpace(query.String())) > 128 {
			return errors.New("query must contain 1 to 128 characters")
		}
		if value := gjson.GetBytes(body, "count"); value.Exists() && (value.Type != gjson.Number || value.Int() < 1 || value.Int() > 50 || value.Float() != float64(value.Int())) {
			return errors.New("count must be an integer from 1 to 50")
		}
		if value := gjson.GetBytes(body, "ori_question"); value.Exists() && value.Type != gjson.Null && (value.Type != gjson.String || utf8.RuneCountInString(value.String()) < 1 || utf8.RuneCountInString(value.String()) > 128) {
			return errors.New("ori_question must be null or contain 1 to 128 characters")
		}
		if value := gjson.GetBytes(body, "localization"); value.Exists() && value.Type != gjson.String {
			return errors.New("localization must be a string")
		}
		if value := gjson.GetBytes(body, "need_content"); value.Exists() && value.Type != gjson.True && value.Type != gjson.False {
			return errors.New("need_content must be a boolean")
		}
		return nil
	case service.DeepSeekToolFetch:
		value := gjson.GetBytes(body, "url")
		if value.Type != gjson.String || utf8.RuneCountInString(value.String()) < 1 || utf8.RuneCountInString(value.String()) > 8192 {
			return errors.New("url must be a non-empty HTTP or HTTPS URL no longer than 8192 characters")
		}
		parsed, err := url.Parse(strings.TrimSpace(value.String()))
		if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
			return errors.New("url must be an HTTP or HTTPS URL with a host and no username or password")
		}
		return nil
	default:
		return errors.New("unsupported DeepSeek tool")
	}
}

func (h *OpenAIGatewayHandler) recordDeepSeekToolUsage(
	c *gin.Context,
	apiKey *service.APIKey,
	account *service.Account,
	subscription *service.UserSubscription,
	model string,
	body []byte,
	result *service.OpenAIForwardResult,
	userID int64,
) {
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	sessionID := service.ExtractClientSessionID(c)
	requestPayloadHash := service.HashUsageRequestPayload(body)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
	h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: requestPayloadHash,
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      quotaPlatform,
			SessionID:          sessionID,
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.deepseek_tools"),
				zap.Int64("user_id", userID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.String("model", model),
				zap.Int64("account_id", account.ID),
			).Error("deepseek_tools.record_usage_failed", zap.Error(err))
		}
	})
}
