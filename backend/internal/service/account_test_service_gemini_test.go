//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCreateGeminiTestPayload_ImageModel(t *testing.T) {
	t.Parallel()

	payload := createGeminiTestPayload("gemini-2.5-flash-image", "draw a tiny robot")

	var parsed struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			ResponseModalities []string `json:"responseModalities"`
			ImageConfig        struct {
				AspectRatio string `json:"aspectRatio"`
			} `json:"imageConfig"`
		} `json:"generationConfig"`
	}

	require.NoError(t, json.Unmarshal(payload, &parsed))
	require.Len(t, parsed.Contents, 1)
	require.Len(t, parsed.Contents[0].Parts, 1)
	require.Equal(t, "draw a tiny robot", parsed.Contents[0].Parts[0].Text)
	require.Equal(t, []string{"TEXT", "IMAGE"}, parsed.GenerationConfig.ResponseModalities)
	require.Equal(t, "1:1", parsed.GenerationConfig.ImageConfig.AspectRatio)
}

func TestProcessGeminiStream_EmitsImageEvent(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"QUJD\"}}]}}]}\n\ndata: [DONE]\n\n")

	err := svc.processGeminiStream(ctx, stream)
	require.NoError(t, err)

	body := recorder.Body.String()
	require.Contains(t, body, "\"type\":\"content\"")
	require.Contains(t, body, "\"text\":\"ok\"")
	require.Contains(t, body, "\"type\":\"image\"")
	require.Contains(t, body, "\"image_url\":\"data:image/png;base64,QUJD\"")
	require.Contains(t, body, "\"mime_type\":\"image/png\"")
}

func TestCreateGeminiTestPayloadForMode_CustomImageModel(t *testing.T) {
	t.Parallel()
	payload := createGeminiTestPayloadForMode("draw a red apple", true)
	require.Equal(t, "draw a red apple", gjson.GetBytes(payload, "contents.0.parts.0.text").String())
	require.Equal(t, "IMAGE", gjson.GetBytes(payload, "generationConfig.responseModalities.1").String())
}

func TestProcessGeminiImageResponse_RequiresActualImage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := &AccountTestService{}
	ctx, recorder := newTestContext()
	err := svc.processGeminiImageResponse(ctx, strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"done"}]}}]}`))
	require.Error(t, err)
	require.NotContains(t, recorder.Body.String(), `"success":true`)

	ctx, recorder = newTestContext()
	var picture bytes.Buffer
	require.NoError(t, png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	imageData := base64.StdEncoding.EncodeToString(picture.Bytes())
	body, marshalErr := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": imageData}}}}}}})
	require.NoError(t, marshalErr)
	require.NoError(t, svc.processGeminiImageResponse(ctx, strings.NewReader(string(body))))
	require.Contains(t, recorder.Body.String(), `"type":"image"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}

func TestGeminiCustomImageProbe_UsesNativeNonStreamingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"no image"}]}}]}`)),
	}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "fixture", "base_url": "https://gemini.example", "model_mapping": map[string]any{"banana": "nano-banana-2"}},
		Extra:       map[string]any{accountImageModelsKey: []string{"banana"}},
	}
	err := svc.testGeminiAccountConnection(ctx, account, "banana", "draw an apple", "default")
	require.Error(t, err, "text-only responses must fail image probes")
	require.Equal(t, "/v1beta/models/nano-banana-2:generateContent", upstream.lastReq.URL.Path)
	require.Empty(t, upstream.lastReq.URL.RawQuery)
	require.Equal(t, "fixture", upstream.lastReq.Header.Get("x-goog-api-key"))
	require.NotContains(t, recorder.Body.String(), `"success":true`)

	req, err := svc.buildGeminiAPIKeyRequest(context.Background(), account, "nano-banana-2", createGeminiTestPayloadForMode("hi", false), false)
	require.NoError(t, err)
	require.Contains(t, req.URL.Path, ":streamGenerateContent")
}

func TestGeminiImageProbe_RejectsCorruptImage(t *testing.T) {
	_, valid := validatedGeminiProbeImage(base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}))
	require.False(t, valid)
	ctx, recorder := newTestContext()
	svc := &AccountTestService{}
	err := svc.processGeminiStream(ctx, strings.NewReader("data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"QUJD\"}}]}}]}\n\n"), true)
	require.Error(t, err)
	require.NotContains(t, recorder.Body.String(), `"success":true`)
}

func TestValidateAccountImageModelsExtra(t *testing.T) {
	for _, value := range []any{"nano", []any{12}, []string{"../invalid"}, make([]string, 51)} {
		require.Error(t, ValidateAccountImageModelsExtra(map[string]any{accountImageModelsKey: value}))
	}
	extra := map[string]any{accountImageModelsKey: []any{" nano-banana-2 ", "nano-banana-2"}}
	require.NoError(t, ValidateAccountImageModelsExtra(extra))
	require.Equal(t, []string{"nano-banana-2"}, extra[accountImageModelsKey])
	require.NoError(t, ValidateAccountImageModelsExtra(map[string]any{accountImageModelsKey: []string{}}))
}

func TestGeminiDefaultTestModel(t *testing.T) {
	t.Parallel()

	googleOne := &Account{Platform: PlatformGemini, Type: AccountTypeOAuth, Credentials: map[string]any{"oauth_type": "google_one"}}
	require.Equal(t, geminicli.GoogleOneModels[0].ID, geminiDefaultTestModel(googleOne))
	require.Contains(t, googleOne.GetModelMapping(), geminiDefaultTestModel(googleOne))

	apiKey := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	require.Equal(t, geminicli.DefaultTestModel, geminiDefaultTestModel(apiKey))
}
