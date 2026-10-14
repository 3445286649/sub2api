//go:build unit

package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

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
	payload := createGeminiTestPayloadForMode("nano-banana-2", "draw a red apple", true)
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
	imageData := base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0})
	body, marshalErr := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": imageData}}}}}}})
	require.NoError(t, marshalErr)
	require.NoError(t, svc.processGeminiImageResponse(ctx, strings.NewReader(string(body))))
	require.Contains(t, recorder.Body.String(), `"type":"image"`)
	require.Contains(t, recorder.Body.String(), `"success":true`)
}
