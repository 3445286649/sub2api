package handler

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type UsageRebateHandler struct {
	service *service.UsageRebateService
}

func NewUsageRebateHandler(rebateService *service.UsageRebateService) *UsageRebateHandler {
	return &UsageRebateHandler{service: rebateService}
}

func (h *UsageRebateHandler) GetOverview(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.service == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage rebate service unavailable")
		return
	}

	ctx := c.Request.Context()
	enabled := h.service.IsEnabled(ctx)
	position := service.UsageRebatePosition{}
	if enabled {
		item, err := h.service.GetUserPosition(ctx, time.Now(), subject.UserID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to load usage rebate position")
			return
		}
		position = item
	}
	rewards, err := h.service.ListMyRewards(ctx, subject.UserID, 30)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to load usage rebate rewards")
		return
	}
	publicRewards := make([]usageRebatePublicReward, 0, len(rewards))
	for _, item := range rewards {
		publicRewards = append(publicRewards, usageRebatePublicReward{
			BusinessDate: item.BusinessDate, Rank: item.Rank,
			SpendAmount: item.SpendAmount, RebatePercent: item.RebatePercent,
			RewardAmount: item.RewardAmount, Status: item.Status,
			CreditedAt: item.CreditedAt,
		})
	}
	response.Success(c, gin.H{
		"enabled":         enabled,
		"business_date":   time.Now().In(usageRebateHandlerLocation()).Format("2006-01-02"),
		"timezone":        "Asia/Shanghai",
		"settlement_time": "00:15",
		"rates":           service.UsageRebateRates(),
		"leaderboard":     []any{},
		"my_position": usageRebatePublicPosition{
			Rank: position.Rank, ParticipantCount: position.ParticipantCount,
			Requests: position.Requests, Tokens: position.Tokens, SpendAmount: position.SpendAmount,
			RebatePercent: position.RebatePercent, EstimatedReward: position.EstimatedReward,
			Eligible: position.Eligible, PreviousRank: position.PreviousRank,
			GapToPrevious: position.GapToPrevious, GapToTop20: position.GapToTop20,
		},
		"my_rewards": publicRewards,
	})
}

type usageRebatePublicPosition struct {
	Rank             *int             `json:"rank"`
	ParticipantCount int              `json:"participant_count"`
	Requests         int64            `json:"requests"`
	Tokens           int64            `json:"tokens"`
	SpendAmount      decimal.Decimal  `json:"spend_amount"`
	RebatePercent    decimal.Decimal  `json:"rebate_percent"`
	EstimatedReward  decimal.Decimal  `json:"estimated_reward"`
	Eligible         bool             `json:"eligible"`
	PreviousRank     *int             `json:"previous_rank"`
	GapToPrevious    *decimal.Decimal `json:"gap_to_previous"`
	GapToTop20       *decimal.Decimal `json:"gap_to_top20"`
}

type usageRebatePublicReward struct {
	BusinessDate  string          `json:"business_date"`
	Rank          int             `json:"rank"`
	SpendAmount   decimal.Decimal `json:"spend_amount"`
	RebatePercent decimal.Decimal `json:"rebate_percent"`
	RewardAmount  decimal.Decimal `json:"reward_amount"`
	Status        string          `json:"status"`
	CreditedAt    *time.Time      `json:"credited_at,omitempty"`
}

func usageRebateHandlerLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return location
}
