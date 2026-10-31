//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIVideoReferenceCapabilities(t *testing.T) {
	refs := make([]string, 30)
	for i := range refs {
		refs[i] = `{"image_url":"https://assets.example/ref.png"}`
	}
	body := `{"model":"video","reference_images":[` + strings.Join(refs, ",") + `],"resolution":"4k"}`
	info, err := ParseOpenAIVideoRequest([]byte(body))
	require.NoError(t, err)
	require.Len(t, info.InputImageURLs, 30)
	require.Equal(t, "4k", info.Resolution)
	_, err = ParseOpenAIVideoRequest([]byte(`{"model":"video","prompt":"test","duration":0}`))
	require.NoError(t, err, "existing clients may use zero to request the upstream default")
	for _, invalid := range []string{
		`{"model":"video","reference_images":[` + strings.Join(append(refs, refs[0]), ",") + `]}`,
		`{"model":"video","last_frame":"https://assets.example/ref.png"}`,
		`{"model":"video","first_frame":"https://assets.example/ref.png","reference_images":["https://assets.example/ref.png"]}`,
		`{"model":"video","reference_videos":["http://169.254.169.254/x"]}`,
		`{"model":"video","reference_audios":[{}]}`,
		`{"model":"video","prompt":"test","duration":1.5}`,
	} {
		_, err := ParseOpenAIVideoRequest([]byte(invalid))
		require.Error(t, err, invalid)
	}
	_, err = ParseOpenAIVideoRequest([]byte(`{"model":"video","first_frame":{"image_url":"https://assets.example/first.png"},"last_frame":"https://assets.example/last.png"}`))
	require.NoError(t, err)
}

func TestOpenAIVideoThirtyImageForwardAndPricing(t *testing.T) {
	account := openAIVideoTestAccount()
	account.Credentials[videoModelCapabilitiesKey] = []VideoModelCapability{{Model: "*", MaxReferenceImages: 30, MaxReferenceVideos: 10, MaxReferenceAudios: 10, SupportsFrames: true, Resolutions: []string{"4k"}}}
	refs := make([]string, 30)
	for i := range refs {
		refs[i] = `{"image_url":"https://assets.example/ref.png"}`
	}
	body := []byte(`{"model":"public-video","reference_images":[` + strings.Join(refs, ",") + `],"resolution":"4k","duration":30,"future_field":{"preserve":true}}`)
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"thirty","status":"queued"}`)}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	result, err := svc.ForwardOpenAIVideo(context.Background(), c, account, OpenAIVideoEndpointCreate, "", body)
	require.NoError(t, err)
	require.Equal(t, "4k", result.VideoResolution)
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(forwarded, &payload))
	require.Len(t, payload["reference_images"], 30)
	require.Equal(t, map[string]any{"preserve": true}, payload["future_field"])
	price := 2.2
	groupID := int64(24)
	group := &Group{ID: groupID, RateMultiplier: 1, ModelPricing: []ChannelModelPricing{{Models: []string{"public-video"}, BillingMode: BillingModeVideo, Intervals: []PricingInterval{{TierLabel: "4k", PerRequestPrice: &price}}}}}
	key := &APIKey{GroupID: &groupID, Group: group, User: &User{ID: 7}}
	billing := NewBillingService(nil, nil)
	svc.resolver = NewModelPricingResolver(nil, billing)
	snapshot := svc.OpenAIVideoPricing(context.Background(), key, "public-video", "4k")
	require.NotNil(t, snapshot)
	require.True(t, snapshot.PerSecond)
	group.ModelPricing = nil // Completion must still use the creation snapshot.
	cost := svc.calculateOpenAIVideoCost(context.Background(), "public-video", key, &OpenAIForwardResult{ResponseID: "openai-video:thirty", VideoCount: 1, VideoDurationSeconds: 30, VideoResolution: "4k", OpenAIVideoPricing: snapshot}, 1)
	require.InDelta(t, 66, cost.ActualCost, 1e-9)
	_, err = ParseOpenAIVideoRequest([]byte(`{"model":"v","reference_videos":["https://assets.example/a.mp4"],"reference_audios":[{"audio_url":"https://assets.example/a.mp3"}]}`))
	require.NoError(t, err)
}

func TestOpenAIVideoAccountModelCapabilities(t *testing.T) {
	a := openAIVideoTestAccount()
	refs := make([]string, 30)
	for i := range refs {
		refs[i] = `"https://assets.example/ref.png"`
	}
	info, err := ParseOpenAIVideoRequest([]byte(`{"model":"public-video","reference_images":[` + strings.Join(refs, ",") + `],"resolution":"4k","duration":5}`))
	require.NoError(t, err)
	require.Error(t, a.ValidateOpenAIVideoRequest(info))
	a.Credentials["video_model_capabilities"] = json.RawMessage(`[{"model":"SD2.0-9图满血版-720P","max_reference_images":30,"max_reference_videos":10,"max_reference_audios":10,"supports_frames":true,"resolutions":["4k"],"min_duration":1,"max_duration":15}]`)
	require.NoError(t, a.ValidateOpenAIVideoRequest(info))
	info.DurationSeconds = 0
	require.Error(t, a.ValidateOpenAIVideoRequest(info))
	info.DurationSeconds = 16
	require.Error(t, a.ValidateOpenAIVideoRequest(info))
	_, err = ParseVideoModelCapabilities(json.RawMessage(`[{"model":"x","max_reference_images":31,"resolutions":["4k"]}]`))
	require.Error(t, err)
}

func TestOpenAIVideoCapabilitiesFilterBeforeSelection(t *testing.T) {
	a := openAIVideoTestAccount()
	a.Status = StatusActive
	a.Schedulable = true
	ctx := WithOpenAIVideoRequest(context.Background(), GrokMediaRequestInfo{ReferenceImageCount: 30, Resolution: "720p"})
	require.False(t, isOpenAICompatibleAccountEligibleForRequest(ctx, a, PlatformOpenAI, "public-video", false, OpenAIEndpointCapabilityVideoTasks))
	require.Error(t, OpenAIVideoCapabilityError(ctx))
	a.Credentials[videoModelCapabilitiesKey] = []VideoModelCapability{{Model: "*", MaxReferenceImages: 30, Resolutions: []string{"720p"}}}
	require.True(t, isOpenAICompatibleAccountEligibleForRequest(ctx, a, PlatformOpenAI, "public-video", false, OpenAIEndpointCapabilityVideoTasks))
	scheduler := &defaultOpenAIAccountScheduler{}
	req := OpenAIAccountScheduleRequest{RequestedModel: "public-video", RequiredCapability: OpenAIEndpointCapabilityVideoTasks}
	require.True(t, scheduler.isAccountRequestCompatible(ctx, a, req))
	a.Credentials[videoModelCapabilitiesKey] = nil
	require.False(t, scheduler.isAccountRequestCompatible(ctx, a, req))
	require.True(t, accountSupportsVideoRequest(context.Background(), a, "public-video", OpenAIEndpointCapabilityVideoTasks), "unrelated requests have no media requirements")
}

func TestOpenAIVideoLimitsAndInvalidAdminRules(t *testing.T) {
	for _, field := range []string{"reference_videos", "reference_audios"} {
		refs := make([]string, 10)
		for i := range refs {
			refs[i] = `"https://assets.example/ref"`
		}
		_, err := ParseOpenAIVideoRequest([]byte(`{"model":"v","` + field + `":[` + strings.Join(refs, ",") + `]}`))
		require.NoError(t, err)
		_, err = ParseOpenAIVideoRequest([]byte(`{"model":"v","` + field + `":[` + strings.Join(append(refs, refs[0]), ",") + `]}`))
		require.Error(t, err)
	}
	credentials := map[string]any{videoModelCapabilitiesKey: []map[string]any{{"model": "*", "max_reference_images": 31, "resolutions": []string{"4k"}}}}
	svc := &adminServiceImpl{}
	_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{Credentials: credentials})
	require.ErrorContains(t, err, "video reference limits")
	_, err = svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{Credentials: credentials})
	require.ErrorContains(t, err, "video reference limits")
	_, err = svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{Credentials: credentials})
	require.ErrorContains(t, err, "video reference limits")
	_, err = ParseVideoModelCapabilities(json.RawMessage(`[{"model":"*","resolutions":["4k"],"supports_frame":true}]`))
	require.Error(t, err, "misspelled settings cannot silently weaken validation")
}

func TestCustomMenuPublicSettingsAndFrameOrigins(t *testing.T) {
	raw := `[{"url":"https://legacy.example/","visibility":"user"},{"url":"https://embed.example/","visibility":"user","open_mode":"embed"},{"url":"https://disabled.example/","visibility":"user","enabled":false},{"url":"https://external.example/","visibility":"user","open_mode":"new_tab"},{"url":"https://admin.example/","visibility":"admin"}]`
	var filtered []map[string]any
	require.NoError(t, json.Unmarshal(filterUserVisibleMenuItems(raw), &filtered))
	require.Len(t, filtered, 3)
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{SettingKeyCustomMenuItems: raw}}, &config.Config{})
	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.NoError(t, err)
	require.Contains(t, origins, "https://legacy.example")
	require.Contains(t, origins, "https://embed.example")
	require.NotContains(t, origins, "https://disabled.example")
	require.NotContains(t, origins, "https://external.example")
}
