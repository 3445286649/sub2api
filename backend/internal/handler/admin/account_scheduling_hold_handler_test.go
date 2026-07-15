package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type schedulingHoldHandlerRepo struct {
	state *service.AccountSchedulingState
}

func (r *schedulingHoldHandlerRepo) GetSchedulingState(context.Context, int64, time.Time) (*service.AccountSchedulingState, error) {
	return r.state, nil
}

func (r *schedulingHoldHandlerRepo) PutSchedulingHold(context.Context, service.AccountSchedulingHoldPut, time.Time) (*service.AccountSchedulingState, error) {
	return r.state, nil
}

func (r *schedulingHoldHandlerRepo) ReleaseSchedulingHold(context.Context, service.AccountSchedulingHoldRelease, time.Time) (*service.AccountSchedulingState, error) {
	return r.state, nil
}

func (r *schedulingHoldHandlerRepo) ExpireSchedulingHolds(context.Context, string, time.Time, int) ([]int64, error) {
	return nil, nil
}

func TestAccountSchedulingHoldCapabilitiesMatchesFrozenFixture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAccountSchedulingHoldHandler(service.NewAccountSchedulingHoldService(&schedulingHoldHandlerRepo{}))
	router := gin.New()
	router.GET("/api/v1/admin/scheduling/capabilities", handler.Capabilities)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/scheduling/capabilities", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	want := readSchedulingContractFixture(t, "upstreamops-scheduling-capabilities-v1.json")
	require.JSONEq(t, string(want), recorder.Body.String())
}

func TestAccountSchedulingHoldStateMatchesFrozenFixture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accountUpdatedAt := mustSchedulingTime(t, "2026-07-15T16:10:00Z")
	leaseUntil := mustSchedulingTime(t, "2026-07-15T16:30:00Z")
	lastCheckedAt := mustSchedulingTime(t, "2026-07-15T16:09:00Z")
	nextProbeAt := mustSchedulingTime(t, "2026-07-15T16:11:00Z")
	repo := &schedulingHoldHandlerRepo{state: &service.AccountSchedulingState{
		AccountID: 749, AccountUpdatedAt: accountUpdatedAt, ManualSchedulable: true,
		InternalReasonCodes: []string{}, EffectiveReasonCodes: []string{"external_hold"}, EffectiveSchedulable: false,
		ExternalHold: &service.AccountSchedulingExternalHold{Owner: service.AccountSchedulingHoldOwner, DecisionID: "ops-749-20260715-001", ReasonCode: service.AccountSchedulingHoldReasonSustainedTTFT, Status: "active", LeaseUntil: leaseUntil, Active: true},
		Health:       &service.AccountSchedulingHealthEvidence{Score: 72, Status: "degraded", LastCheckedAt: &lastCheckedAt, NextProbeAt: &nextProbeAt, ProbeEnabled: true},
	}}
	handler := NewAccountSchedulingHoldHandler(service.NewAccountSchedulingHoldService(repo))
	router := gin.New()
	router.GET("/api/v1/admin/accounts/:id/scheduling-state", handler.GetState)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/749/scheduling-state", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	want := readSchedulingContractFixture(t, "upstreamops-scheduling-state-v1.json")
	require.JSONEq(t, string(want), recorder.Body.String())
}

func TestAccountSchedulingHoldPutRejectsMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAccountSchedulingHoldHandler(service.NewAccountSchedulingHoldService(&schedulingHoldHandlerRepo{}))
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id/scheduling-holds/upstreamops", handler.Put)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/7/scheduling-holds/upstreamops", bytes.NewBufferString(`{"decision_id":`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "INVALID_HOLD_REQUEST", body["reason"])
}

func readSchedulingContractFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../../../docs/contracts/" + name)
	require.NoError(t, err)
	return raw
}

func mustSchedulingTime(t *testing.T, raw string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	return value
}
