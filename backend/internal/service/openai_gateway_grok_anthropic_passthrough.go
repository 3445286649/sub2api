package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
)

func (s *OpenAIGatewayService) forwardGrokAnthropicMessagesAPIKeyPassthrough(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := strings.TrimSpace(anthropicReq.Model)
	if originalModel == "" {
		return nil, fmt.Errorf("missing model in request")
	}
	requestModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	requestModel = normalizeOpenAIModelForUpstream(account, requestModel)
	if requestModel != originalModel {
		body = s.ReplaceModelInBody(body, requestModel)
	}

	gateway := &GatewayService{
		accountRepo:           s.accountRepo,
		usageLogRepo:          s.usageLogRepo,
		usageBillingRepo:      s.usageBillingRepo,
		userRepo:              s.userRepo,
		userSubRepo:           s.userSubRepo,
		cache:                 s.cache,
		cfg:                   s.cfg,
		billingService:        s.billingService,
		rateLimitService:      s.rateLimitService,
		billingCacheService:   s.billingCacheService,
		httpUpstream:          s.httpUpstream,
		deferredService:       s.deferredService,
		settingService:        s.settingService,
		accountHealthService:  s.accountHealthService,
		responseHeaderFilter:  s.responseHeaderFilter,
		channelService:        s.channelService,
		resolver:              s.resolver,
		balanceNotifyService:  s.balanceNotifyService,
		userPlatformQuotaRepo: s.userPlatformQuotaRepo,
	}
	result, err := gateway.forwardAnthropicAPIKeyPassthrough(ctx, c, account, body, requestModel, originalModel, anthropicReq.Stream, startTime)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	return &OpenAIForwardResult{
		RequestID: result.RequestID,
		Usage: OpenAIUsage{
			InputTokens:              result.Usage.InputTokens,
			OutputTokens:             result.Usage.OutputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
			ImageOutputTokens:        result.Usage.ImageOutputTokens,
		},
		Model:            result.Model,
		BillingModel:     originalModel,
		UpstreamModel:    result.UpstreamModel,
		Stream:           result.Stream,
		Duration:         result.Duration,
		FirstTokenMs:     result.FirstTokenMs,
		ClientDisconnect: result.ClientDisconnect,
		ReasoningEffort:  result.ReasoningEffort,
	}, nil
}
