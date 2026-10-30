//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIVideoLookupUsesOwnerAndBoundAccount(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	taskID := service.OpenAIVideoTaskKey("task-1")
	groupID := int64(24)
	require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(context.Background(), taskID, 10, 20, service.GrokVideoPendingBilling{
		GroupID: groupID, AccountID: 1, Model: "public-video", BillingModel: "public-video",
		OpenAIVideoPricing: &service.OpenAIVideoPricingSnapshot{UnitPrice: 0.42, RateMultiplier: 1},
	}))
	bindings.writes = 0
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		require.Equal(t, int64(1), id)
		require.Equal(t, "/api/v3/v1/videos/task-1", req.URL.Path)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"task-1","status":"queued"}`))}, nil
	}
	newContext := func() (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), false)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		key.Group.Status = service.StatusActive
		key.Group.RateMultiplier = 1
		price := 0.42
		key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"public-video"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}}
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID, nil)
		c.Params = gin.Params{{Key: "request_id", Value: taskID}}
		return c, w
	}
	c, w := newContext()
	h.OpenAIVideoStatus(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), taskID)
	slots.assertReleased(t)
	for _, mutate := range []func(*gin.Context){
		func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		},
		func(c *gin.Context) { key, _ := middleware.GetAPIKeyFromContext(c); key.ID = 21 },
		func(c *gin.Context) { key, _ := middleware.GetAPIKeyFromContext(c); id := int64(25); key.GroupID = &id },
	} {
		c, w = newContext()
		mutate(c)
		before := upstream.calls
		h.OpenAIVideoStatus(c)
		require.Equal(t, 404, w.Code)
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	require.Zero(t, bindings.writes)
	c, _ = newContext()
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	completed := &service.OpenAIForwardResult{ResponseID: taskID, VideoCount: 1}
	require.NotNil(t, prepareOpenAIVideoCompletionBilling(context.Background(), h, key, subject, taskID, completed))
	require.Nil(t, prepareOpenAIVideoCompletionBilling(context.Background(), h, key, subject, taskID, completed))
}

func TestOpenAIVideoCreateStoresTaskBeforeResponse(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	upstream.call = func(req *http.Request, _ int64) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/api/v3/v1/videos", req.URL.Path)
		return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"task-new","status":"queued"}`))}, nil
	}
	c, w := grokMediaSlotContext(context.Background(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.Group.Platform = service.PlatformOpenAI
	key.Group.Status = service.StatusActive
	key.Group.RateMultiplier = 1
	price := 0.42
	key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"gpt-5.4"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"gpt-5.4","prompt":"nine cats"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.OpenAIVideoGeneration(c)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"id":"openai-video:task-new"`)
	require.Len(t, bindings.pending, 1)
	require.Equal(t, 1, upstream.calls)
	slots.assertReleased(t)
}

func TestOpenAIVideoCreateUsesResolutionTierWithoutDefault(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	upstream.call = func(req *http.Request, _ int64) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"task-tier","status":"queued"}`))}, nil
	}
	c, w := grokMediaSlotContext(context.Background(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.Group.Platform = service.PlatformOpenAI
	key.Group.Status = service.StatusActive
	key.Group.RateMultiplier = 1
	price := 5.8
	key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"gpt-5.4"}, BillingMode: service.BillingModePerRequest,
		Intervals: []service.PricingInterval{{TierLabel: "1080p", PerRequestPrice: &price}}}}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"gpt-5.4","prompt":"nine cats","resolution":"1080p"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.OpenAIVideoGeneration(c)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	require.Equal(t, 1, upstream.calls)
	require.Len(t, bindings.pending, 1)
	for _, pending := range bindings.pending {
		require.InDelta(t, price, gjson.GetBytes(pending, "openai_video_pricing.unit_price").Float(), 1e-9)
		require.Equal(t, "1080p", gjson.GetBytes(pending, "video_resolution").String())
	}
	slots.assertReleased(t)
}

func TestOpenAIVideoCreateDoesNotReturnUnboundTask(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	bindings.pendingError = true
	upstream.call = func(*http.Request, int64) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"id":"task-new","status":"queued"}`))}, nil
	}
	c, w := grokMediaSlotContext(context.Background(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.Group.Platform = service.PlatformOpenAI
	key.Group.Status = service.StatusActive
	key.Group.RateMultiplier = 1
	price := 0.42
	key.Group.ModelPricing = []service.ChannelModelPricing{{Models: []string{"gpt-5.4"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"gpt-5.4","prompt":"nine cats"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.OpenAIVideoGeneration(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.NotContains(t, w.Body.String(), "task-new")
	require.Equal(t, 1, upstream.calls, "ambiguous create must not retry")
	slots.assertReleased(t)
}
