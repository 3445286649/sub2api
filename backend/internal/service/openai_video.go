package service

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	OpenAIVideoEndpointCreate          GrokMediaEndpoint        = "openai_video_create"
	OpenAIVideoEndpointStatus          GrokMediaEndpoint        = "openai_video_status"
	OpenAIVideoEndpointContent         GrokMediaEndpoint        = "openai_video_content"
	OpenAIEndpointCapabilityVideoTasks OpenAIEndpointCapability = "video_tasks"
)

func (e GrokMediaEndpoint) IsOpenAIVideo() bool {
	return e == OpenAIVideoEndpointCreate || e == OpenAIVideoEndpointStatus || e == OpenAIVideoEndpointContent
}

func OpenAIVideoTaskKey(id string) string { return "openai-video:" + strings.TrimSpace(id) }

func (s *OpenAIGatewayService) OpenAIVideoTaskSubscription(ctx context.Context, subscriptionID, userID, groupID int64) (*UserSubscription, error) {
	if s == nil || s.userSubRepo == nil || subscriptionID <= 0 {
		return nil, fmt.Errorf("video subscription repository is unavailable")
	}
	sub, err := s.userSubRepo.GetByID(ctx, subscriptionID)
	if err != nil || sub == nil || sub.UserID != userID || sub.GroupID != groupID {
		return nil, fmt.Errorf("video task subscription not found")
	}
	return sub, nil
}

type OpenAIVideoPricingSnapshot struct {
	UnitPrice      float64 `json:"unit_price"`
	RateMultiplier float64 `json:"rate_multiplier"`
	PerSecond      bool    `json:"per_second,omitempty"`
}

func (s *OpenAIGatewayService) OpenAIVideoPricing(ctx context.Context, key *APIKey, model, resolution string) *OpenAIVideoPricingSnapshot {
	if s == nil || s.resolver == nil || key == nil || key.Group == nil || key.GroupID == nil || key.User == nil {
		return nil
	}
	resolution, ok := LookupOpenAIVideoResolution(resolution)
	if !ok {
		return nil
	}
	pricing := s.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: key.GroupID, Group: key.Group})
	if pricing == nil || (pricing.Source != PricingSourceGroup && pricing.Source != PricingSourceChannel) ||
		(pricing.Mode != BillingModePerRequest && pricing.Mode != BillingModeVideo) || pricing.channelPricing == nil {
		return nil
	}
	price := pricing.channelPricing.PerRequestPrice
	for _, tier := range pricing.RequestTiers {
		if tierResolution, valid := LookupOpenAIVideoResolution(tier.TierLabel); valid && tierResolution == resolution {
			price = tier.PerRequestPrice
			break
		}
	}
	if price == nil || *price < 0 || math.IsNaN(*price) || math.IsInf(*price, 0) {
		return nil
	}
	rate := 1.0
	if s.cfg != nil {
		rate = s.cfg.Default.RateMultiplier
	}
	rate = s.ResolveUserGroupRateMultiplier(ctx, key.User.ID, *key.GroupID, key.Group.RateMultiplier)
	rate = resolveVideoRateMultiplier(key, rate)
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil
	}
	return &OpenAIVideoPricingSnapshot{UnitPrice: *price, RateMultiplier: rate, PerSecond: pricing.Mode == BillingModeVideo}
}

func ParseOpenAIVideoRequest(body []byte) (GrokMediaRequestInfo, error) {
	var info GrokMediaRequestInfo
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("request body must be a JSON object")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.String()) == "" || len(model.String()) > 256 {
		return info, fmt.Errorf("model is required (max 256 bytes)")
	}
	info.Model = strings.TrimSpace(model.String())
	prompt := gjson.GetBytes(body, "prompt")
	if prompt.Exists() && prompt.Type != gjson.String {
		return info, fmt.Errorf("prompt must be a string")
	}
	info.Prompt = strings.TrimSpace(prompt.String())
	for _, field := range []struct {
		name, key string
		limit     int
		count     *int
	}{
		{"reference_images", "image_url", 30, &info.ReferenceImageCount},
		{"reference_videos", "video_url", 10, &info.ReferenceVideoCount},
		{"reference_audios", "audio_url", 10, &info.ReferenceAudioCount},
	} {
		refs := gjson.GetBytes(body, field.name)
		if !refs.Exists() {
			continue
		}
		if !refs.IsArray() || len(refs.Array()) > field.limit {
			return info, fmt.Errorf("%s must be an array with at most %d entries", field.name, field.limit)
		}
		*field.count = len(refs.Array())
		for _, ref := range refs.Array() {
			raw := ref
			if ref.IsObject() {
				raw = ref.Get(field.key)
			}
			if raw.Type != gjson.String || !isPublicVideoReferenceURL(raw.String()) {
				return info, fmt.Errorf("%s require public HTTP(S) file URLs", field.name)
			}
			if field.name == "reference_images" {
				info.InputImageURLs = append(info.InputImageURLs, raw.String())
			}
		}
	}
	for _, field := range []string{"first_frame", "last_frame"} {
		ref := gjson.GetBytes(body, field)
		if !ref.Exists() {
			continue
		}
		raw := ref
		if ref.IsObject() {
			raw = ref.Get("image_url")
		}
		if raw.Type != gjson.String || !isPublicVideoReferenceURL(raw.String()) {
			return info, fmt.Errorf("%s requires a public HTTP(S) image URL", field)
		}
		info.HasVideoFrames = true
		info.InputImageURLs = append(info.InputImageURLs, raw.String())
	}
	if gjson.GetBytes(body, "last_frame").Exists() && !gjson.GetBytes(body, "first_frame").Exists() {
		return info, fmt.Errorf("last_frame requires first_frame")
	}
	if info.HasVideoFrames && info.ReferenceImageCount+info.ReferenceVideoCount+info.ReferenceAudioCount > 0 {
		return info, fmt.Errorf("first/last frames cannot be combined with reference materials")
	}
	if info.Prompt == "" && len(info.InputImageURLs)+info.ReferenceVideoCount+info.ReferenceAudioCount == 0 {
		return info, fmt.Errorf("prompt or reference material is required")
	}
	info.Resolution = VideoBillingResolution480P
	if requestedResolution := gjson.GetBytes(body, "resolution"); requestedResolution.Exists() {
		if requestedResolution.Type != gjson.String {
			return info, fmt.Errorf("resolution must be 480p, 720p, 1080p or 4k")
		}
		var valid bool
		info.Resolution, valid = LookupOpenAIVideoResolution(requestedResolution.String())
		if !valid {
			return info, fmt.Errorf("resolution must be 480p, 720p, 1080p or 4k")
		}
	}
	if duration := gjson.GetBytes(body, "duration"); duration.Exists() {
		if duration.Type != gjson.Number || duration.Float() != float64(duration.Int()) || duration.Int() < 0 || duration.Int() > 600 {
			return info, fmt.Errorf("duration must be an integer from 0 to 600 seconds")
		}
		info.DurationSeconds = int(duration.Int())
	}
	if ratio := gjson.GetBytes(body, "ratio"); ratio.Exists() {
		if ratio.Type != gjson.String {
			return info, fmt.Errorf("ratio must be a string")
		}
		switch ratio.String() {
		case "auto", "adaptive", "16:9", "9:16", "1:1", "4:3", "3:4", "21:9":
		default:
			return info, fmt.Errorf("unsupported video ratio")
		}
	}
	return info, nil
}

func isPublicVideoReferenceURL(raw string) bool {
	if len(raw) > 8192 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") {
		return false
	}
	if strings.Trim(host, "0123456789.") == "" && net.ParseIP(host) == nil {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return false
	}
	return true
}

func buildOpenAIVideoURL(base string, endpoint GrokMediaEndpoint, taskID string) (string, error) {
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	base += "/videos"
	if endpoint != OpenAIVideoEndpointCreate {
		if taskID == "" || validateUpstreamPathSegment("video task ID", taskID) != nil {
			return "", fmt.Errorf("invalid video task ID")
		}
		base += "/" + taskID
		if endpoint == OpenAIVideoEndpointContent {
			base += "/content"
		}
	}
	return base, nil
}

// ForwardOpenAIVideo preserves the upstream task protocol and rewrites only the mapped model and public task ID.
func (s *OpenAIGatewayService) ForwardOpenAIVideo(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (forwardResult *OpenAIForwardResult, forwardErr error) {
	defer func() { stampGrokRejection(c, forwardResult) }()
	if account == nil || !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoTasks) || !endpoint.IsOpenAIVideo() {
		return nil, fmt.Errorf("OpenAI video tasks require an enabled API key account")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	upstreamID := strings.TrimPrefix(taskID, "openai-video:")
	target, err := buildOpenAIVideoURL(base, endpoint, upstreamID)
	if err != nil {
		return nil, err
	}
	model, upstreamModel := "", ""
	var createInfo GrokMediaRequestInfo
	if endpoint == OpenAIVideoEndpointCreate {
		info, parseErr := ParseOpenAIVideoRequest(body)
		if parseErr != nil {
			return nil, parseErr
		}
		createInfo = info
		if err := account.ValidateOpenAIVideoRequest(info); err != nil {
			return nil, err
		}
		model = info.Model
		upstreamModel = account.GetMappedModel(model)
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("video account missing api_key")
	}
	method := http.MethodGet
	if endpoint == OpenAIVideoEndpointCreate {
		method = http.MethodPost
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if endpoint == OpenAIVideoEndpointContent {
		req.Header.Set("Accept", "*/*")
		if c != nil {
			if value := strings.TrimSpace(c.GetHeader("Range")); value != "" {
				req.Header.Set("Range", value)
			}
		}
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if endpoint == OpenAIVideoEndpointCreate {
		req.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.doOpenAIUpstream(req, proxy, account, c)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if endpoint == OpenAIVideoEndpointContent && resp.StatusCode < 300 {
		if _, err := writeGrokMediaContentResponse(c, resp); err != nil {
			return nil, err
		}
		videoCount := 0
		mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent) && resp.ContentLength != 0 &&
			(strings.HasPrefix(mediaType, "video/") || mediaType == "application/octet-stream") {
			videoCount = 1
		}
		return &OpenAIForwardResult{ResponseID: taskID, VideoCount: videoCount, Duration: time.Since(started)}, nil
	}
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("video upstream status %d", resp.StatusCode)
	}
	result := &OpenAIForwardResult{Model: model, BillingModel: model, UpstreamModel: upstreamModel, ResponseID: taskID, Duration: time.Since(started), ResponseHeaders: resp.Header.Clone()}
	if endpoint == OpenAIVideoEndpointCreate {
		result.VideoResolution = createInfo.Resolution
		result.VideoDurationSeconds = createInfo.DurationSeconds
		id := strings.TrimSpace(gjson.GetBytes(responseBody, "id").String())
		if id == "" || validateUpstreamPathSegment("video task ID", id) != nil {
			return nil, fmt.Errorf("video create response missing valid task ID")
		}
		result.ResponseID = OpenAIVideoTaskKey(id)
		responseBody, err = sjson.SetBytes(responseBody, "id", result.ResponseID)
		if err != nil {
			return nil, err
		}
		result.DeferredResponseBody = responseBody
		result.DeferredResponseStatus = resp.StatusCode
		return result, nil
	} else if endpoint == OpenAIVideoEndpointStatus {
		if strings.EqualFold(gjson.GetBytes(responseBody, "status").String(), "completed") {
			result.VideoCount = 1
		}
		result.UpstreamModel = gjson.GetBytes(responseBody, "model").String()
		if duration := gjson.GetBytes(responseBody, "duration"); duration.Type == gjson.Number {
			result.VideoDurationSeconds = int(duration.Int())
		}
		responseBody, err = sjson.SetBytes(responseBody, "id", taskID)
		if err != nil {
			return nil, err
		}
		responseBody = rewriteGrokMediaVideoContentURLs(responseBody, upstreamID, grokMediaContentProxyURL(c, taskID))
	}
	c.Data(resp.StatusCode, "application/json", responseBody)
	return result, nil
}
