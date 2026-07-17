package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type usageRebateHandlerRepoStub struct{}

func (usageRebateHandlerRepoStub) EnsureOpenPeriod(context.Context, service.UsageRebatePeriodSeed) error {
	return nil
}
func (usageRebateHandlerRepoStub) ClaimDuePeriod(context.Context, time.Time, time.Time) (*service.UsageRebatePeriod, error) {
	return nil, nil
}
func (usageRebateHandlerRepoStub) SealClaimedPeriod(context.Context, int64, []service.UsageRebateRate) error {
	return nil
}
func (usageRebateHandlerRepoStub) ListPayableRewards(context.Context, int64) ([]service.UsageRebateReward, error) {
	return nil, nil
}
func (usageRebateHandlerRepoStub) CreditReward(context.Context, int64) (int64, bool, error) {
	return 0, false, nil
}
func (usageRebateHandlerRepoStub) MarkRewardFailed(context.Context, int64, string) error {
	return nil
}
func (usageRebateHandlerRepoStub) MarkRewardUnknown(context.Context, int64, string) error {
	return nil
}
func (usageRebateHandlerRepoStub) FinalizePeriod(context.Context, int64) error { return nil }
func (usageRebateHandlerRepoStub) MarkPeriodFailed(context.Context, int64, string) error {
	return nil
}
func (usageRebateHandlerRepoStub) GetLeaderboard(context.Context, time.Time, time.Time, int64, int) ([]service.UsageRebateCandidate, error) {
	return []service.UsageRebateCandidate{{
		UserID: 7, Username: "viewer", Rank: 1, Requests: 8, Tokens: 900,
		SpendAmount: decimal.NewFromInt(36), RebatePercent: decimal.NewFromInt(10),
		EstimatedReward: decimal.RequireFromString("3.6"),
	}}, nil
}
func (usageRebateHandlerRepoStub) ListUserRewards(context.Context, int64, int) ([]service.UsageRebateReward, error) {
	return []service.UsageRebateReward{{
		ID: 9, PeriodID: 3, UserID: 7, BusinessDate: "2026-07-17", Rank: 1,
		SpendAmount: decimal.NewFromInt(36), RebatePercent: decimal.NewFromInt(10),
		RewardAmount: decimal.RequireFromString("3.6"), Status: service.UsageRebateRewardStatusCredited,
		BalanceBefore: decimal.NewFromInt(10), BalanceAfter: decimal.RequireFromString("13.6"),
		ErrorMessage: "must not leak", CreatedAt: time.Now(),
	}}, nil
}
func (usageRebateHandlerRepoStub) ListRecentPeriods(context.Context, int) ([]service.UsageRebatePeriod, error) {
	return nil, nil
}
func (usageRebateHandlerRepoStub) ListPeriodRewards(context.Context, int64, int) ([]service.UsageRebateReward, error) {
	return nil, nil
}

type usageRebateEnabledStub struct{}

func (usageRebateEnabledStub) IsUsageRebateEnabled(context.Context) bool { return true }

func TestUsageRebateUserRouteIsReadOnlyAndRedactsInternalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewUsageRebateService(usageRebateHandlerRepoStub{}, usageRebateEnabledStub{})
	handler := NewUsageRebateHandler(svc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 7})
		c.Next()
	})
	router.GET("/api/v1/usage-rebate", handler.GetOverview)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/usage-rebate", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	encoded := recorder.Body.String()
	require.NotContains(t, encoded, "user_id")
	require.NotContains(t, encoded, "period_id")
	require.NotContains(t, encoded, "balance_before")
	require.NotContains(t, encoded, "balance_after")
	require.NotContains(t, encoded, "error_message")
	require.Contains(t, encoded, `"is_me":true`)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mutation := httptest.NewRequest(method, "/api/v1/usage-rebate", strings.NewReader(`{"rank":1,"reward_amount":999}`))
		mutation.Header.Set("Content-Type", "application/json")
		mutationRecorder := httptest.NewRecorder()
		router.ServeHTTP(mutationRecorder, mutation)
		require.Equal(t, http.StatusNotFound, mutationRecorder.Code, method)
	}
}
