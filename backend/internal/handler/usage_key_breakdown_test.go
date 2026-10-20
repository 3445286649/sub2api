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

type keyBreakdownRepoStub struct {
	service.UsageLogRepository
	query  usagestats.APIKeyBreakdownQuery
	called bool
}

func (r *keyBreakdownRepoStub) GetUserAPIKeyBreakdown(_ context.Context, q usagestats.APIKeyBreakdownQuery) (*usagestats.APIKeyUsageBreakdown, error) {
	r.called, r.query = true, q
	if q.UserID != 42 || q.APIKeyID != 7 {
		return nil, service.ErrAPIKeyNotFound
	}
	return &usagestats.APIKeyUsageBreakdown{
		APIKeyID: 7, Name: "deleted-work", Deleted: true, StartTime: q.StartTime, EndTime: q.EndTime,
		Groups: []usagestats.APIKeyGroupUsage{},
	}, nil
}

func breakdownRouter(repo *keyBreakdownRepoStub, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) { c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42}) })
	}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	r.GET("/api-keys/:id/breakdown", h.APIKeyBreakdown)
	return r
}

func TestAPIKeyBreakdownScopesToSessionAndReturnsDeletedKey(t *testing.T) {
	repo := &keyBreakdownRepoStub{}
	w := httptest.NewRecorder()
	breakdownRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/api-keys/7/breakdown?"+summaryRange+"&user_id=99&api_key_id=8&group_id=3", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(42), repo.query.UserID)
	require.Equal(t, int64(7), repo.query.APIKeyID)
	require.Equal(t, int64(3), repo.query.GroupID)
	require.Equal(t, 24*time.Hour, repo.query.EndTime.Sub(repo.query.StartTime))
	require.Equal(t, 42, repo.query.StartTime.Minute())
	var body struct {
		Data usagestats.APIKeyUsageBreakdown `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.EqualValues(t, 7, body.Data.APIKeyID)
	require.True(t, body.Data.Deleted)
	require.Equal(t, repo.query.StartTime, body.Data.StartTime)
	require.Empty(t, body.Data.Groups)
	require.Contains(t, w.Body.String(), `"groups":[]`)
	for _, privateField := range []string{`"key":`, "account_cost", "upstream_model", "total_cost"} {
		require.NotContains(t, w.Body.String(), privateField)
	}
}

func TestAPIKeyBreakdownRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name, id, query string
		auth            bool
		status          int
	}{
		{"unauthenticated", "7", summaryRange, false, 401},
		{"missing range", "7", "", true, 400},
		{"bad date", "7", "start_time=bad&end_time=2026-10-19T00:00:00Z", true, 400},
		{"reversed range", "7", "start_time=2026-10-19T00:00:00Z&end_time=2026-10-18T00:00:00Z", true, 400},
		{"equal range", "7", "start_time=2026-10-19T00:00:00Z&end_time=2026-10-19T00:00:00Z", true, 400},
		{"overlong range", "7", "start_time=2025-01-01T00:00:00Z&end_time=2026-10-19T00:00:00Z", true, 400},
		{"negative key", "-1", summaryRange, true, 400},
		{"zero key", "0", summaryRange, true, 400},
		{"invalid key", "bad", summaryRange, true, 400},
		{"overflow key", "9223372036854775808", summaryRange, true, 400},
		{"injected key", "7%3BOR%201%3D1", summaryRange, true, 400},
		{"invalid group", "7", summaryRange + "&group_id=bad", true, 400},
		{"zero group", "7", summaryRange + "&group_id=0", true, 400},
		{"negative group", "7", summaryRange + "&group_id=-1", true, 400},
		{"injected group", "7", summaryRange + "&group_id=3%3BOR%201%3D1", true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &keyBreakdownRepoStub{}
			w := httptest.NewRecorder()
			breakdownRouter(repo, tc.auth).ServeHTTP(w, httptest.NewRequest("GET", "/api-keys/"+tc.id+"/breakdown?"+tc.query, nil))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.False(t, repo.called)
		})
	}
}

func TestAPIKeyBreakdownRejectsForeignOrMissingKey(t *testing.T) {
	for _, id := range []string{"8", "999"} {
		repo := &keyBreakdownRepoStub{}
		w := httptest.NewRecorder()
		breakdownRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/api-keys/"+id+"/breakdown?"+summaryRange, nil))
		require.Equal(t, http.StatusNotFound, w.Code)
		require.EqualValues(t, 42, repo.query.UserID)
	}
}
