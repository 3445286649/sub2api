package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiForwardAsChatCompletions_AccountsForAliasImageOutputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const model = "nano-banana-2.1"
	const imagePart = `{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}`
	protocols := []struct {
		name        string
		accountType string
		stream      bool
	}{
		{"json", AccountTypeAPIKey, false},
		{"sse", AccountTypeAPIKey, true},
		{"oauth aggregated sse", AccountTypeOAuth, false},
		{"oauth sse", AccountTypeOAuth, true},
	}
	outputs := []struct {
		name  string
		parts string
		count int
	}{
		{"one image", imagePart, 1},
		{"two images", imagePart + "," + imagePart, 2},
		{"text only", `{"text":"hello"}`, 0},
	}
	for _, protocol := range protocols {
		for _, output := range outputs {
			t.Run(protocol.name+"/"+output.name, func(t *testing.T) {
				payload := `{"candidates":[{"content":{"parts":[` + output.parts + `]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":3}}`
				if protocol.accountType == AccountTypeOAuth {
					payload = `{"response":` + payload + `}`
				}
				contentType := "application/json"
				if protocol.stream || protocol.accountType == AccountTypeOAuth {
					payload = "data: " + payload + "\n\n"
					if protocol.stream {
						// Repeated cumulative chunks must not charge for the same images twice.
						payload += payload
					}
					payload += "data: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				httpStub := &geminiCompatHTTPUpstreamStub{response: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{contentType}},
					Body:       io.NopCloser(strings.NewReader(payload)),
				}}
				svc := &GeminiMessagesCompatService{
					httpUpstream: httpStub, cfg: &config.Config{}, tokenProvider: &GeminiTokenProvider{},
				}
				account := &Account{
					ID: 1, Platform: PlatformGemini, Type: protocol.accountType, Concurrency: 1,
					Credentials: map[string]any{
						"api_key": "test-key", "access_token": "test-token", "project_id": "test-project",
						"model_mapping": map[string]any{model: "custom-image-upstream"},
					},
				}
				body, err := json.Marshal(map[string]any{
					"model": model, "stream": protocol.stream,
					"messages": []any{map[string]any{"role": "user", "content": "draw a cat"}},
				})
				require.NoError(t, err)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
				// A prior forwarding attempt on the same context must not leak its count.
				beginGeminiImageOutputObservation(c)
				observeGeminiImageOutputs(c, []byte(`{"candidates":[{"content":{"parts":[`+imagePart+`]}}]}`))

				result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, 1, httpStub.calls)
				require.Equal(t, output.count, result.ImageCount)
				require.Equal(t, 7, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
				var content string
				if protocol.stream {
					for _, line := range strings.Split(rec.Body.String(), "\n") {
						if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
							continue
						}
						var chunk struct {
							Choices []struct {
								Delta struct {
									Content string `json:"content"`
								} `json:"delta"`
							} `json:"choices"`
						}
						require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
						for _, choice := range chunk.Choices {
							content += choice.Delta.Content
						}
					}
				} else {
					var response struct {
						Choices []struct {
							Message struct {
								Content string `json:"content"`
							} `json:"message"`
						} `json:"choices"`
					}
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
					require.Len(t, response.Choices, 1)
					content = response.Choices[0].Message.Content
				}
				require.Equal(t, output.count, strings.Count(content, "![image](data:image/png;base64,aW1hZ2U=)"))
				if output.count == 0 {
					require.Equal(t, "hello", content)
					return
				}

				groupID := int64(117)
				price := 0.10
				cache := newEmptyChannelCache()
				cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}] = &ChannelModelPricing{
					BillingMode: BillingModePerRequest, PerRequestPrice: &price,
				}
				cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
				cache.loadedAt = time.Now()
				channels := &ChannelService{}
				channels.cache.Store(cache)
				billing := NewBillingService(&config.Config{}, nil)
				gateway := &GatewayService{billingService: billing, resolver: NewModelPricingResolver(channels, billing)}
				cost := gateway.calculateRecordUsageCost(context.Background(), result,
					&APIKey{GroupID: &groupID, Group: &Group{ID: groupID}}, model, 1, 1, time.Time{})
				require.NotNil(t, cost)
				require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
				require.InDelta(t, float64(output.count)*price, cost.ActualCost, 1e-12)
			})
		}
	}
}

func TestGeminiResponseToChatCompletionsPreservesInlineData(t *testing.T) {
	tests := []struct {
		name  string
		parts []any
		want  string
	}{
		{
			name: "image only",
			parts: []any{
				map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "aW1hZ2U="}},
			},
			want: "![image](data:image/png;base64,aW1hZ2U=)",
		},
		{
			name: "text and image",
			parts: []any{
				map[string]any{"text": "rendered image:\n"},
				map[string]any{"inlineData": map[string]any{"mimeType": "image/webp", "data": "d2VicA=="}},
			},
			want: "rendered image:\n![image](data:image/webp;base64,d2VicA==)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geminiResp := map[string]any{
				"candidates": []any{map[string]any{
					"content":      map[string]any{"parts": tt.parts},
					"finishReason": "STOP",
				}},
			}
			rawData, err := json.Marshal(geminiResp)
			require.NoError(t, err)

			got, _, err := geminiResponseToChatCompletions(geminiResp, "gemini-test", rawData, nil)
			require.NoError(t, err)
			require.Len(t, got.Choices, 1)

			var content string
			require.NoError(t, json.Unmarshal(got.Choices[0].Message.Content, &content))
			require.Equal(t, tt.want, content)
			require.Equal(t, "stop", got.Choices[0].FinishReason)
		})
	}
}

func TestGeminiResponseToChatCompletionsOmitsInvalidInlineData(t *testing.T) {
	tests := []struct {
		name       string
		inlineData map[string]any
	}{
		{
			name:       "unsupported MIME type",
			inlineData: map[string]any{"mimeType": "image/svg+xml", "data": "PHN2Zz48L3N2Zz4="},
		},
		{
			name:       "malformed MIME type",
			inlineData: map[string]any{"mimeType": "image/png; charset=utf-8", "data": "aW1hZ2U="},
		},
		{
			name:       "malformed base64",
			inlineData: map[string]any{"mimeType": "image/png", "data": "not-valid-base64!!!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geminiResp := map[string]any{
				"candidates": []any{map[string]any{
					"content":      map[string]any{"parts": []any{map[string]any{"text": "before"}, map[string]any{"inlineData": tt.inlineData}, map[string]any{"text": "after"}}},
					"finishReason": "STOP",
				}},
			}
			rawData, err := json.Marshal(geminiResp)
			require.NoError(t, err)

			got, _, err := geminiResponseToChatCompletions(geminiResp, "gemini-test", rawData, nil)
			require.NoError(t, err)

			var content string
			require.NoError(t, json.Unmarshal(got.Choices[0].Message.Content, &content))
			require.Equal(t, "beforeafter", content)
		})
	}
}

func TestConvertGeminiToClaudeMessageOmitsInlineDataForAnthropicMessages(t *testing.T) {
	geminiResp := map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{
				map[string]any{"text": "before"},
				map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": "aW1hZ2U="}},
				map[string]any{"functionCall": map[string]any{"name": "get_weather", "args": map[string]any{"city": "Paris"}}},
				map[string]any{"text": "after"},
			}},
			"finishReason": "STOP",
		}},
	}
	rawData, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	withInlineData, _ := convertGeminiToClaudeMessage(geminiResp, "gemini-test", rawData, true)
	contentWithInlineData, ok := withInlineData["content"].([]any)
	require.True(t, ok)
	require.Len(t, contentWithInlineData, 4)
	require.Equal(t, map[string]any{"type": "text", "text": "before"}, contentWithInlineData[0])
	require.Equal(t, map[string]any{"type": "text", "text": "![image](data:image/png;base64,aW1hZ2U=)"}, contentWithInlineData[1])
	toolUse, ok := contentWithInlineData[2].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_use", toolUse["type"])
	require.Equal(t, "get_weather", toolUse["name"])
	require.Equal(t, map[string]any{"type": "text", "text": "after"}, contentWithInlineData[3])

	withoutInlineData, _ := convertGeminiToClaudeMessage(geminiResp, "gemini-test", rawData, false)
	contentWithoutInlineData, ok := withoutInlineData["content"].([]any)
	require.True(t, ok)
	require.Len(t, contentWithoutInlineData, 3)
	require.Equal(t, map[string]any{"type": "text", "text": "before"}, contentWithoutInlineData[0])
	toolUseWithoutInlineData, ok := contentWithoutInlineData[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_use", toolUseWithoutInlineData["type"])
	require.Equal(t, "get_weather", toolUseWithoutInlineData["name"])
	require.Equal(t, map[string]any{"type": "text", "text": "after"}, contentWithoutInlineData[2])
}

func TestGeminiResponseToChatCompletionsRetainsTextAndToolBehavior(t *testing.T) {
	geminiResp := map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{
				map[string]any{"text": "checking"},
				map[string]any{"functionCall": map[string]any{
					"name": "get_weather",
					"args": map[string]any{"city": "Paris"},
				}},
			}},
			"finishReason": "STOP",
		}},
	}
	rawData, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	got, _, err := geminiResponseToChatCompletions(geminiResp, "gemini-test", rawData, nil)
	require.NoError(t, err)
	require.Len(t, got.Choices, 1)

	choice := got.Choices[0]
	var content string
	require.NoError(t, json.Unmarshal(choice.Message.Content, &content))
	require.Equal(t, "checking", content)
	require.Equal(t, "tool_calls", choice.FinishReason)
	require.Len(t, choice.Message.ToolCalls, 1)
	require.Equal(t, "get_weather", choice.Message.ToolCalls[0].Function.Name)
	require.JSONEq(t, `{"city":"Paris"}`, choice.Message.ToolCalls[0].Function.Arguments)
}
