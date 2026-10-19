package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type keySummaryRepoStub struct {
	service.UsageLogRepository
	query  usagestats.APIKeySummaryQuery
	called bool
}

func (r *keySummaryRepoStub) GetUserAPIKeySummary(_ context.Context, q usagestats.APIKeySummaryQuery) (*usagestats.APIKeyUsageSummary, error) {
	r.called = true
	r.query = q
	return &usagestats.APIKeyUsageSummary{Items: []usagestats.APIKeyUsageSummaryItem{}, Page: q.Page, PageSize: q.PageSize}, nil
}
func (r *keySummaryRepoStub) OwnsHistoricalAPIKey(_ context.Context, userID, keyID int64) (bool, error) {
	return userID == 42 && keyID == 7, nil
}

func summaryRouter(repo *keySummaryRepoStub, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) { c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42}) })
	}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	r.GET("/summary", h.APIKeySummary)
	return r
}

const summaryRange = "start_time=2026-10-18T15:42:00Z&end_time=2026-10-19T15:42:00Z"

func TestAPIKeySummaryScopesToSessionAndKeepsExactTime(t *testing.T) {
	repo := &keySummaryRepoStub{}
	w := httptest.NewRecorder()
	summaryRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/summary?"+summaryRange+"&user_id=99&api_key_id=8&group_id=3&page=2&page_size=10&only_used=true&sort_by=total_tokens", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(42), repo.query.UserID)
	require.Equal(t, int64(8), repo.query.APIKeyID)
	require.Equal(t, int64(3), repo.query.GroupID)
	require.Equal(t, 24*time.Hour, repo.query.EndTime.Sub(repo.query.StartTime))
	require.Equal(t, 42, repo.query.StartTime.Minute())
	require.Equal(t, 2, repo.query.Page)
	require.True(t, repo.query.OnlyUsed)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotContains(t, w.Body.String(), "account_cost")
}

func TestAPIKeySummaryRejectsUnauthenticatedAndInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		query  string
		auth   bool
		status int
	}{
		{summaryRange, false, 401}, {"", true, 400},
		{"start_time=2026-10-19T00:00:00Z&end_time=2026-10-18T00:00:00Z", true, 400},
		{"start_time=2025-01-01T00:00:00Z&end_time=2026-10-19T00:00:00Z", true, 400},
		{summaryRange + "&page=0", true, 400}, {summaryRange + "&page_size=101", true, 400},
		{summaryRange + "&sort_by=name%3BDROP", true, 400}, {summaryRange + "&sort_order=foo", true, 400},
		{summaryRange + "&api_key_id=-1", true, 400}, {summaryRange + "&group_id=bad", true, 400},
		{summaryRange + "&only_used=bad", true, 400},
	} {
		t.Run(tc.query, func(t *testing.T) {
			repo := &keySummaryRepoStub{}
			w := httptest.NewRecorder()
			summaryRouter(repo, tc.auth).ServeHTTP(w, httptest.NewRequest("GET", "/summary?"+tc.query, nil))
			require.Equal(t, tc.status, w.Code)
			require.False(t, repo.called)
		})
	}
}

func TestUserUsageExactRangeOverridesDates(t *testing.T) {
	repo := &userUsageRepoCapture{}
	w := httptest.NewRecorder()
	newUserUsageRequestTypeTestRouter(repo).ServeHTTP(w, httptest.NewRequest("GET", "/usage?"+summaryRange+"&start_date=2026-10-18&end_date=2026-10-19", nil))
	require.Equal(t, 200, w.Code)
	require.Equal(t, 24*time.Hour, repo.listFilters.EndTime.Sub(*repo.listFilters.StartTime))
	require.Equal(t, 42, repo.listFilters.StartTime.Minute())
}

func TestUserUsageDeletedKeyRemainsOwnerScoped(t *testing.T) {
	repo := &keySummaryRepoStub{}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), service.NewAPIKeyService(&dailyUsageAPIKeyRepoStub{keys: map[int64]*service.APIKey{}}, nil, nil, nil, nil, nil, nil), nil, nil)
	for _, id := range []string{"7", "8"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Request = httptest.NewRequest("GET", "/usage?api_key_id="+id, nil)
		parsed, ok := h.parseUserUsageFilters(c, false)
		if id == "7" {
			require.True(t, ok)
			require.Equal(t, int64(42), parsed.Filters.UserID)
			require.Equal(t, int64(7), parsed.Filters.APIKeyID)
		} else {
			require.False(t, ok)
		}
	}
}
