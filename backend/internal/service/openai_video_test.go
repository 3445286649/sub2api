//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func openAIVideoTestAccount() *Account {
	return &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "video-secret", "base_url": "https://video.example/v1",
		"openai_capabilities": []string{"video_tasks"},
		"model_mapping":       map[string]any{"public-video": "SD2.0-9图满血版-720P"},
	}}
}

func TestOpenAIVideoForwardLifecycle(t *testing.T) {
	images := make([]string, 9)
	for i := range images {
		images[i] = `{"image_url":"https://assets.example/cat` + string(rune('1'+i)) + `.png"}`
	}
	body := []byte(`{"model":"public-video","prompt":"nine cats","reference_images":[` + strings.Join(images, ",") + `],"future_field":{"keep":true}}`)
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	result, err := svc.ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointCreate, "", body)
	require.NoError(t, err)
	require.Equal(t, "openai-video:task-1", result.ResponseID)
	require.Empty(t, w.Body.String(), "create response must wait for local task binding")
	require.Equal(t, "openai-video:task-1", gjson.GetBytes(result.DeferredResponseBody, "id").String())
	require.Equal(t, 200, result.DeferredResponseStatus)
	require.Equal(t, "https://video.example/v1/videos", upstream.request.URL.String())
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	require.Equal(t, "SD2.0-9图满血版-720P", gjson.GetBytes(forwarded, "model").String())
	require.Len(t, gjson.GetBytes(forwarded, "reference_images").Array(), 9)
	require.Equal(t, gjson.GetBytes(body, "future_field").Raw, gjson.GetBytes(forwarded, "future_field").Raw)

	upstream.response = grokMediaContentStatusResponse(`{"id":"task-1","status":"completed","model":"SD2.0-9图满血版-720P","video":{"url":"https://video.example/v1/videos/task-1/content"}}`)
	c, w = grokMediaContentTestContext(http.MethodGet, "/v1/videos/openai-video:task-1", nil)
	result, err = svc.ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointStatus, "openai-video:task-1", nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "openai-video:task-1", gjson.Get(w.Body.String(), "id").String())
	require.Contains(t, gjson.Get(w.Body.String(), "video.url").String(), "/videos/openai-video:")
	require.Equal(t, "/v1/videos/task-1", upstream.request.URL.Path)

	upstream.response = &http.Response{StatusCode: http.StatusOK, ContentLength: 8, Header: http.Header{"Content-Type": []string{"video/mp4"}},
		Body: io.NopCloser(strings.NewReader("mp4-data"))}
	c, w = grokMediaContentTestContext(http.MethodGet, "/v1/videos/openai-video:task-1/content", nil)
	c.Request.Header.Set("Range", "bytes=0-7")
	result, err = svc.ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointContent, "openai-video:task-1", nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "mp4-data", w.Body.String())
	require.Equal(t, "bytes=0-7", upstream.request.Header.Get("Range"))
	require.Equal(t, "*/*", upstream.request.Header.Get("Accept"))
}

func TestOpenAIVideoRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		`{`, `[]`, `{}`, `{"model":"video","prompt":""}`,
		`{"model":"video","prompt":"ok","resolution":"8k"}`,
		`{"model":"video","prompt":"ok","resolution":1080}`,
		`{"model":"video","prompt":"ok","reference_images":[{"image_url":"http://127.0.0.1/x.png"}]}`,
		`{"model":"video","prompt":"ok","reference_images":[{"image_url":"http://127.1/x.png"}]}`,
		`{"model":"video","prompt":"ok","reference_images":[{"image_url":"file:///x.png"}]}`,
	} {
		_, err := ParseOpenAIVideoRequest([]byte(body))
		require.Error(t, err, body)
	}
	for _, id := range []string{"", "..", "a/b", "a?b", "%2e%2e"} {
		_, err := buildOpenAIVideoURL("https://video.example", OpenAIVideoEndpointStatus, id)
		require.Error(t, err, id)
	}
	a := openAIVideoTestAccount()
	require.True(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoTasks))
	delete(a.Credentials, "openai_capabilities")
	require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoTasks))
}

func TestOpenAIVideoRequiresExplicitPerRequestPrice(t *testing.T) {
	price := 0.42
	groupID := int64(24)
	group := &Group{ID: groupID, RateMultiplier: 1, ModelPricing: []ChannelModelPricing{{
		Models: []string{"public-video"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price,
	}}}
	key := &APIKey{GroupID: &groupID, Group: group, User: &User{ID: 7}}
	billing := NewBillingService(nil, nil)
	svc := &OpenAIGatewayService{resolver: NewModelPricingResolver(nil, billing), billingService: billing}
	snapshot := svc.OpenAIVideoPricing(context.Background(), key, "public-video", "720p")
	require.NotNil(t, snapshot)
	require.InDelta(t, price, snapshot.UnitPrice, 1e-9)
	require.InDelta(t, 1, snapshot.RateMultiplier, 1e-9)
	changedPrice := 3.0
	group.ModelPricing[0].PerRequestPrice = &changedPrice
	group.RateMultiplier = 2
	videoPrice := 9.0
	group.VideoPrice720P = &videoPrice
	cost := svc.calculateOpenAIVideoCost(context.Background(), "public-video", key, &OpenAIForwardResult{
		ResponseID: OpenAIVideoTaskKey("task-1"), VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 8, OpenAIVideoPricing: snapshot,
	}, 1)
	require.InDelta(t, price, cost.ActualCost, 1e-9)
	group.ModelPricing = nil
	result := &OpenAIForwardResult{ResponseID: OpenAIVideoTaskKey("task-1"), VideoCount: 1, OpenAIVideoPricing: snapshot}
	recorded, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, key, []string{"public-video"}, 1, 1, 1, 1, UsageTokens{}, "", nil, false, time.Now())
	require.NoError(t, err)
	require.InDelta(t, price, recorded.ActualCost, 1e-9, "pricing changes after create must not reprice the task")
	group.ModelPricing = []ChannelModelPricing{{Models: []string{"public-video"}, BillingMode: BillingModePerRequest, PerRequestPrice: &changedPrice}}
	require.Nil(t, svc.OpenAIVideoPricing(context.Background(), key, "other-video", "720p"))
	group.ModelPricing[0].PerRequestPrice = nil
	require.Nil(t, svc.OpenAIVideoPricing(context.Background(), key, "public-video", "720p"))
	negative := -1.0
	group.ModelPricing[0].PerRequestPrice = &negative
	require.Nil(t, svc.OpenAIVideoPricing(context.Background(), key, "public-video", "720p"))
}

func TestOpenAIVideoPricingUsesResolutionTierWithoutDefault(t *testing.T) {
	groupID := int64(24)
	prices := []float64{4.8, 5.3, 5.8}
	group := &Group{ID: groupID, RateMultiplier: 1, ModelPricing: []ChannelModelPricing{{
		Models: []string{"public-video"}, BillingMode: BillingModePerRequest,
		Intervals: []PricingInterval{
			{TierLabel: "480p", PerRequestPrice: &prices[0]},
			{TierLabel: "720p", PerRequestPrice: &prices[1]},
			{TierLabel: "1080p", PerRequestPrice: &prices[2]},
		},
	}}}
	key := &APIKey{GroupID: &groupID, Group: group, User: &User{ID: 7}}
	billing := NewBillingService(nil, nil)
	svc := &OpenAIGatewayService{resolver: NewModelPricingResolver(nil, billing), billingService: billing}
	for _, tc := range []struct {
		resolution string
		want       float64
	}{{"480p", 4.8}, {"720p", 5.3}, {"1080p", 5.8}} {
		snapshot := svc.OpenAIVideoPricing(context.Background(), key, "public-video", tc.resolution)
		require.NotNil(t, snapshot, tc.resolution)
		require.InDelta(t, tc.want, snapshot.UnitPrice, 1e-9)
	}
	require.Nil(t, svc.OpenAIVideoPricing(context.Background(), key, "public-video", "4k"))
	group.ModelPricing[0].Intervals = group.ModelPricing[0].Intervals[:2]
	require.Nil(t, svc.OpenAIVideoPricing(context.Background(), key, "public-video", "1080p"))
}

func TestOpenAIVideoCreateDoesNotRetryUpstreamError(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"error":{"message":"quota exceeded"}}`)}
	upstream.response.StatusCode = http.StatusTooManyRequests
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, w := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	_, err := svc.ForwardOpenAIVideo(context.Background(), c, openAIVideoTestAccount(), OpenAIVideoEndpointCreate, "", []byte(`{"model":"public-video","prompt":"cat"}`))
	require.Error(t, err)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Len(t, upstream.requests, 1)
}
