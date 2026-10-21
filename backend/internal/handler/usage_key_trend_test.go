package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const prevRange = "&prev_start_time=2026-10-17T15:42:00Z&prev_end_time=2026-10-18T15:42:00Z"

func TestAPIKeySummaryParsesTrendOptions(t *testing.T) {
	repo := &keySummaryRepoStub{}
	w := httptest.NewRecorder()
	summaryRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/summary?"+summaryRange+prevRange+"&granularity=hour&timezone=Asia%2FShanghai", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.query.PrevStartTime)
	require.Equal(t, 24*time.Hour, repo.query.StartTime.Sub(*repo.query.PrevStartTime))
	require.Equal(t, "hour", repo.query.Granularity)
	require.Equal(t, "Asia/Shanghai", repo.query.Location.String())
}

func TestAPIKeySummaryTrendOptionsDefaultOff(t *testing.T) {
	repo := &keySummaryRepoStub{}
	w := httptest.NewRecorder()
	summaryRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/summary?"+summaryRange, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, repo.query.PrevStartTime)
	require.Empty(t, repo.query.Granularity)
}

func TestKeyUsageTrendOptionsRejectInvalid(t *testing.T) {
	for _, q := range []string{
		"&prev_start_time=2026-10-17T15:42:00Z",
		"&prev_start_time=2026-10-18T00:00:00Z&prev_end_time=2026-10-19T00:00:00Z",
		"&prev_start_time=2026-10-18T00:00:00Z&prev_end_time=2026-10-17T00:00:00Z",
		"&prev_start_time=2025-01-01T00:00:00Z&prev_end_time=2026-10-18T00:00:00Z",
		"&granularity=minute",
		"&timezone=Not%2FAZone", "&timezone=Local",
	} {
		t.Run(q, func(t *testing.T) {
			for _, path := range []string{"/summary?", "/api-keys/7/breakdown?"} {
				w := httptest.NewRecorder()
				if path == "/summary?" {
					repo := &keySummaryRepoStub{}
					summaryRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", path+summaryRange+q, nil))
					require.False(t, repo.called)
				} else {
					repo := &keyBreakdownRepoStub{}
					breakdownRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", path+summaryRange+q, nil))
					require.False(t, repo.called)
				}
				require.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestKeyUsageHourGranularityLimitedToTwoDays(t *testing.T) {
	repo := &keyBreakdownRepoStub{}
	w := httptest.NewRecorder()
	breakdownRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/api-keys/7/breakdown?start_time=2026-10-15T00:00:00Z&end_time=2026-10-18T00:00:00Z&granularity=hour", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	w = httptest.NewRecorder()
	breakdownRouter(repo, true).ServeHTTP(w, httptest.NewRequest("GET", "/api-keys/7/breakdown?start_time=2026-10-16T00:00:00Z&end_time=2026-10-18T00:00:00Z&granularity=hour", nil))
	require.Equal(t, http.StatusOK, w.Code)
}
