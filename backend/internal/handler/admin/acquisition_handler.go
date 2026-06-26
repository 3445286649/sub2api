package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AcquisitionHandler struct {
	acquisitionService *service.AcquisitionService
}

func NewAcquisitionHandler(acquisitionService *service.AcquisitionService) *AcquisitionHandler {
	return &AcquisitionHandler{acquisitionService: acquisitionService}
}

type acquisitionCampaignRequest struct {
	Name                string                                  `json:"name"`
	Status              string                                  `json:"status"`
	StartsAt            time.Time                               `json:"starts_at" binding:"required"`
	EndsAt              time.Time                               `json:"ends_at" binding:"required"`
	LeaderboardEnabled  bool                                    `json:"leaderboard_enabled"`
	LotteryEnabled      bool                                    `json:"lottery_enabled"`
	LeaderboardPoolUSD  float64                                 `json:"leaderboard_pool_usd"`
	LeaderboardShares   []float64                               `json:"leaderboard_shares"`
	LotteryPrizeConfigs []service.AcquisitionLotteryPrizeConfig `json:"lottery_prize_configs"`
	LotterySeed         string                                  `json:"lottery_seed"`
}

func (r acquisitionCampaignRequest) toServiceInput() service.AcquisitionCampaignInput {
	return service.AcquisitionCampaignInput{
		Name:                r.Name,
		Status:              r.Status,
		StartsAt:            r.StartsAt,
		EndsAt:              r.EndsAt,
		LeaderboardEnabled:  r.LeaderboardEnabled,
		LotteryEnabled:      r.LotteryEnabled,
		LeaderboardPoolUSD:  r.LeaderboardPoolUSD,
		LeaderboardShares:   r.LeaderboardShares,
		LotteryPrizeConfigs: r.LotteryPrizeConfigs,
		LotterySeed:         r.LotterySeed,
	}
}

// ListCampaigns lists acquisition campaigns.
// GET /api/v1/admin/acquisition/campaigns?status=active
func (h *AcquisitionHandler) ListCampaigns(c *gin.Context) {
	filter := service.AcquisitionCampaignFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  100,
	}
	items, err := h.acquisitionService.ListCampaigns(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// GetCampaign returns campaign detail with leaderboard and rewards.
// GET /api/v1/admin/acquisition/campaigns/:id
func (h *AcquisitionHandler) GetCampaign(c *gin.Context) {
	id, ok := parseAcquisitionCampaignID(c)
	if !ok {
		return
	}
	detail, err := h.acquisitionService.GetCampaignDetail(c.Request.Context(), id, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// CreateCampaign creates a campaign.
// POST /api/v1/admin/acquisition/campaigns
func (h *AcquisitionHandler) CreateCampaign(c *gin.Context) {
	var req acquisitionCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.acquisitionService.CreateCampaign(c.Request.Context(), req.toServiceInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

// UpdateCampaign updates a draft/active campaign.
// PUT /api/v1/admin/acquisition/campaigns/:id
func (h *AcquisitionHandler) UpdateCampaign(c *gin.Context) {
	id, ok := parseAcquisitionCampaignID(c)
	if !ok {
		return
	}
	var req acquisitionCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.acquisitionService.UpdateCampaign(c.Request.Context(), id, req.toServiceInput())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// SettleCampaign retries or forces settlement for a campaign.
// POST /api/v1/admin/acquisition/campaigns/:id/settle
func (h *AcquisitionHandler) SettleCampaign(c *gin.Context) {
	id, ok := parseAcquisitionCampaignID(c)
	if !ok {
		return
	}
	if err := h.acquisitionService.SettleCampaignNow(c.Request.Context(), id, time.Now().UTC()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

func parseAcquisitionCampaignID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid campaign id")
		return 0, false
	}
	return id, true
}
