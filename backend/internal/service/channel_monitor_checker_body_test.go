//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// swapMonitorHTTPClient 临时替换 monitorHTTPClient 为不带 SSRF 校验的普通 client，
// 让 httptest (127.0.0.1) 能连通。测试结束后恢复。
func swapMonitorHTTPClient(t *testing.T) {
	t.Helper()
	orig := monitorHTTPClient
	monitorHTTPClient = &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(func() { monitorHTTPClient = orig })
}

// captureHandler 把每次收到的请求 body 和 headers 存起来，测试断言用。
type captureHandler struct {
	lastBody             map[string]any
	lastHeaders          http.Header
	respondText          string // 写到 Anthropic content[0].text 里（校验用）
	answerChallenge      bool
	leadingNonTextBlocks bool
	status               int
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.lastHeaders = r.Header.Clone()
	defer func() { _ = r.Body.Close() }()
	var parsed map[string]any
	_ = json.NewDecoder(r.Body).Decode(&parsed)
	h.lastBody = parsed

	if h.status == 0 {
		h.status = 200
	}
	respondText := h.respondText
	if h.answerChallenge {
		respondText = answerFromAnthropicRequest(parsed)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	content := []map[string]any{}
	if h.leadingNonTextBlocks {
		content = append(content,
			map[string]any{"type": "thinking", "thinking": "internal"},
			map[string]any{"type": "tool_use", "name": "noop"},
		)
	}
	content = append(content, map[string]any{"type": "text", "text": respondText})
	// 构造 Anthropic 格式的响应。
	_ = json.NewEncoder(w).Encode(map[string]any{
		"content": content,
	})
}

func setupFakeAnthropic(t *testing.T, handler *captureHandler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

type openAICaptureHandler struct {
	lastBody                  map[string]any
	lastHeaders               http.Header
	seenHeaders               []http.Header
	lastPath                  string
	status                    int
	statusSequence            []int
	callCount                 int
	emptyResponseUntil        int
	errorBody                 string
	rawResponse               string
	responsesLeadingReasoning bool
	responsesSSEMode          string
}

func (h *openAICaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	h.lastHeaders = r.Header.Clone()
	h.seenHeaders = append(h.seenHeaders, r.Header.Clone())
	h.lastPath = r.URL.Path
	defer func() { _ = r.Body.Close() }()
	var parsed map[string]any
	_ = json.NewDecoder(r.Body).Decode(&parsed)
	h.lastBody = parsed

	status := h.status
	if len(h.statusSequence) >= h.callCount {
		status = h.statusSequence[h.callCount-1]
	}
	if status == 0 {
		status = http.StatusOK
	}
	if h.responsesSSEMode != "" && status >= 200 && status < 300 && h.lastPath == providerOpenAIResponsesPath {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		h.writeResponsesSSE(w, answerFromOpenAIRequest(parsed))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if h.callCount == 1 {
		w.Header().Set(ChannelMonitorHeaderSelectedAccount, "774")
	}
	w.WriteHeader(status)

	if h.rawResponse != "" {
		_, _ = w.Write([]byte(h.rawResponse))
		return
	}
	if status < 200 || status >= 300 {
		if h.errorBody != "" {
			_, _ = w.Write([]byte(h.errorBody))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": http.StatusText(status)},
		})
		return
	}

	answer := answerFromOpenAIRequest(parsed)
	if h.emptyResponseUntil >= h.callCount {
		answer = ""
	}
	if h.lastPath == providerOpenAIResponsesPath {
		output := []map[string]any{}
		if h.responsesLeadingReasoning {
			output = append(output, map[string]any{
				"type":    "reasoning",
				"summary": []any{},
			})
		}
		output = append(output, map[string]any{
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": answer},
			},
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": output,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": answer}}},
	})
}

func (h *openAICaptureHandler) writeResponsesSSE(w http.ResponseWriter, answer string) {
	switch h.responsesSSEMode {
	case "delta":
		mid := len(answer) / 2
		_, _ = w.Write([]byte("data: " + mustMonitorJSON(map[string]any{"type": "response.output_text.delta", "delta": answer[:mid]}) + "\n\n"))
		_, _ = w.Write([]byte("data: " + mustMonitorJSON(map[string]any{"type": "response.output_text.delta", "delta": answer[mid:]}) + "\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"))
	case "completed":
		_, _ = w.Write([]byte("data: " + mustMonitorJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"status": "completed",
				"output": []map[string]any{{
					"type":    "message",
					"content": []map[string]any{{"type": "output_text", "text": answer}},
				}},
			},
		}) + "\r\n\r\n"))
	case "failed":
		_, _ = w.Write([]byte("data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"upstream_error\",\"message\":\"account unavailable\"}}}\n\n"))
	case "truncated":
		_, _ = w.Write([]byte("data: " + mustMonitorJSON(map[string]any{"type": "response.output_text.delta", "delta": answer}) + "\n\n"))
	}
}

func mustMonitorJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func setupFakeOpenAI(t *testing.T, handler *openAICaptureHandler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

func answerFromOpenAIRequest(body map[string]any) string {
	prompt, _ := body["input"].(string)
	if prompt == "" {
		if messages, ok := body["messages"].([]any); ok && len(messages) > 0 {
			if msg, ok := messages[0].(map[string]any); ok {
				prompt, _ = msg["content"].(string)
			}
		}
	}
	return answerFromChallengePrompt(prompt)
}

func answerFromAnthropicRequest(body map[string]any) string {
	if messages, ok := body["messages"].([]any); ok && len(messages) > 0 {
		if msg, ok := messages[0].(map[string]any); ok {
			if prompt, _ := msg["content"].(string); prompt != "" {
				return answerFromChallengePrompt(prompt)
			}
		}
	}
	return "0"
}

var challengeQuestionRegex = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\nA:$`)

func answerFromChallengePrompt(prompt string) string {
	m := challengeQuestionRegex.FindStringSubmatch(prompt)
	if len(m) != 4 {
		return "0"
	}
	left, _ := strconv.Atoi(m[1])
	right, _ := strconv.Atoi(m[3])
	if m[2] == "+" {
		return strconv.Itoa(left + right)
	}
	return strconv.Itoa(left - right)
}

func TestRunCheckForModel_OffMode_PreservesDefaultBody(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	// 跑一次 off 模式（opts=nil），确认默认 body 行为未变
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", nil)

	if h.lastBody["model"] != "claude-x" {
		t.Errorf("default body should contain model=claude-x, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("default body should contain messages")
	}
	if h.lastHeaders.Get("x-api-key") != "sk-fake" {
		t.Errorf("expected adapter's x-api-key header, got %q", h.lastHeaders.Get("x-api-key"))
	}
}

func TestRunCheckForModel_OpenAI_DefaultChatRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("default chat request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIPath {
		t.Fatalf("expected chat completions path %q, got %q", providerOpenAIPath, h.lastPath)
	}
	if h.lastBody["model"] != "gpt-test" {
		t.Errorf("chat body should contain model=gpt-test, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("chat body should contain messages")
	}
	if _, ok := h.lastBody["instructions"]; ok {
		t.Error("chat body must not contain top-level instructions")
	}
	if _, ok := h.lastBody["max_output_tokens"]; ok {
		t.Error("chat completions body must not contain max_output_tokens")
	}
	if _, ok := h.lastBody["max_tokens"]; !ok {
		t.Error("chat completions body should contain max_tokens")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("chat body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer sk-openai" {
		t.Errorf("expected bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestGrokMonitorConfiguration(t *testing.T) {
	if err := validateProvider(MonitorProviderGrok); err != nil {
		t.Fatalf("grok provider should be supported: %v", err)
	}
	if got := normalizeMonitorPrimaryModel(MonitorProviderGrok, ""); got != MonitorDefaultGrokModel {
		t.Fatalf("expected default Grok model %q, got %q", MonitorDefaultGrokModel, got)
	}
	if err := validateAPIMode(MonitorProviderGrok, MonitorAPIModeChatCompletions); err != nil {
		t.Fatalf("grok chat_completions mode should be valid: %v", err)
	}
	if err := validateAPIMode(MonitorProviderGrok, MonitorAPIModeResponses); err != nil {
		t.Fatalf("grok responses mode should be valid: %v", err)
	}
	if err := validateReplaceRequestBody(MonitorProviderGrok, MonitorAPIModeChatCompletions, map[string]any{}); err == nil {
		t.Fatal("grok replace-mode body should require messages")
	}
}

func TestRunCheckForModel_Grok_DefaultChatRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-key", MonitorDefaultGrokModel, nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("Grok request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if res.LatencyMs == nil {
		t.Fatal("Grok request should record latency")
	}
	if h.lastPath != providerGrokPath {
		t.Fatalf("expected Grok chat completions path %q, got %q", providerGrokPath, h.lastPath)
	}
	if h.lastBody["model"] != MonitorDefaultGrokModel {
		t.Errorf("Grok body should contain model=%s, got %v", MonitorDefaultGrokModel, h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("Grok body should contain messages")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("Grok body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer xai-key" {
		t.Errorf("expected Grok bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestRunCheckForModel_Grok_DefaultResponsesRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-key", MonitorDefaultGrokModel, &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("Grok responses request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected Grok responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
	if h.lastBody["model"] != MonitorDefaultGrokModel {
		t.Errorf("Grok responses body should contain model=%s, got %v", MonitorDefaultGrokModel, h.lastBody["model"])
	}
	if _, ok := h.lastBody["input"]; !ok {
		t.Error("Grok responses body should contain input")
	}
	if _, ok := h.lastBody["instructions"]; !ok {
		t.Error("Grok responses body should contain instructions")
	}
	if _, ok := h.lastBody["messages"]; ok {
		t.Error("Grok responses body must not contain chat messages")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("Grok responses body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer xai-key" {
		t.Errorf("expected Grok bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestRunCheckForModel_Grok_UpstreamFailure(t *testing.T) {
	h := &openAICaptureHandler{status: http.StatusTooManyRequests}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "xai-key", MonitorDefaultGrokModel, nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("Grok 429 should be recorded as error, got status=%s message=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "upstream HTTP 429") {
		t.Fatalf("Grok failure should preserve upstream status, got %q", res.Message)
	}
	if res.LatencyMs == nil {
		t.Fatal("Grok failure should still record latency")
	}
}

func TestRunCheckForModel_Grok_RedactsXAIKeyFromUpstreamBody(t *testing.T) {
	h := &openAICaptureHandler{
		status:      http.StatusUnauthorized,
		rawResponse: `{"error":{"message":"invalid API key xai-secret"}}`,
	}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderGrok, endpoint, "request-key", MonitorDefaultGrokModel, nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("Grok upstream failure should be recorded as error, got %s", res.Status)
	}
	if strings.Contains(res.Message, "xai-secret") {
		t.Fatalf("Grok error message leaked xAI key: %q", res.Message)
	}
	if !strings.Contains(res.Message, "xai-***REDACTED***") {
		t.Fatalf("Grok error message should contain redaction marker, got %q", res.Message)
	}
}

func TestRunCheckForModel_OpenAIResponses_DefaultRequest(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("default responses request should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
	if h.lastBody["model"] != "gpt-test" {
		t.Errorf("responses body should contain model=gpt-test, got %v", h.lastBody["model"])
	}
	instructions, _ := h.lastBody["instructions"].(string)
	if strings.TrimSpace(instructions) == "" {
		t.Error("responses body should contain non-empty instructions")
	}
	input, _ := h.lastBody["input"].(string)
	if strings.TrimSpace(input) == "" {
		t.Error("responses body should contain non-empty input")
	}
	if _, ok := h.lastBody["messages"]; ok {
		t.Error("responses body must not contain chat messages")
	}
	if _, ok := h.lastBody["max_tokens"]; ok {
		t.Error("responses body must not contain max_tokens")
	}
	if _, ok := h.lastBody["max_output_tokens"]; !ok {
		t.Error("responses body should contain max_output_tokens")
	}
	if h.lastBody["stream"] != false {
		t.Errorf("responses body should set stream=false, got %v", h.lastBody["stream"])
	}
	if h.lastHeaders.Get("Authorization") != "Bearer sk-openai" {
		t.Errorf("expected bearer auth header, got %q", h.lastHeaders.Get("Authorization"))
	}
}

func TestRunCheckForModel_OpenAIResponses_StreamMergeAggregatesSSE(t *testing.T) {
	h := &openAICaptureHandler{responsesSSEMode: "delta"}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.6-luna", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"stream":            true,
			"max_output_tokens": float64(128),
		},
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("streaming responses request should pass challenge, got status=%s category=%s message=%q", res.Status, res.FailureCategory, res.Message)
	}
	if h.lastBody["stream"] != true {
		t.Fatalf("stream override should reach upstream, got %v", h.lastBody["stream"])
	}
	if h.lastBody["model"] != "gpt-5.6-luna" {
		t.Fatalf("merge mode should preserve routed model, got %v", h.lastBody["model"])
	}
	if h.lastHeaders.Get("Accept") != "application/json, text/event-stream" {
		t.Fatalf("streaming responses should advertise SSE support, got %q", h.lastHeaders.Get("Accept"))
	}
}

func TestRunCheckForModel_OpenAIResponses_StreamCompletedResponseFallback(t *testing.T) {
	h := &openAICaptureHandler{responsesSSEMode: "completed"}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.6-luna", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride:     map[string]any{"stream": true},
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("completed event response output should pass challenge, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_OpenAIResponses_StreamReplaceUsesFullTemplate(t *testing.T) {
	h := &openAICaptureHandler{responsesSSEMode: "completed"}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "ignored-model", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"model":             "gpt-5.6-luna",
			"instructions":      "Reply briefly.",
			"input":             "Reply with exactly: ok",
			"max_output_tokens": float64(128),
			"stream":            true,
		},
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("streaming replace template should accept non-empty completed output, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastBody["model"] != "gpt-5.6-luna" || h.lastBody["stream"] != true {
		t.Fatalf("replace template should reach upstream unchanged, got body=%v", h.lastBody)
	}
}

func TestRunCheckForModel_OpenAIResponses_StreamFailureIsNotOperational(t *testing.T) {
	h := &openAICaptureHandler{responsesSSEMode: "failed"}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.6-luna", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride:     map[string]any{"stream": true},
	})

	if res.Status != MonitorStatusError || res.FailureCategory != MonitorFailureUpstreamError {
		t.Fatalf("response.failed should be upstream_error, got status=%s category=%s message=%q", res.Status, res.FailureCategory, res.Message)
	}
	if !strings.Contains(res.Message, "account unavailable") {
		t.Fatalf("response.failed message should be preserved, got %q", res.Message)
	}
}

func TestRunCheckForModel_OpenAIResponses_TruncatedStreamIsProtocolError(t *testing.T) {
	h := &openAICaptureHandler{responsesSSEMode: "truncated"}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.6-luna", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride:     map[string]any{"stream": true},
	})

	if res.Status != MonitorStatusError || res.FailureCategory != MonitorFailureProtocolError {
		t.Fatalf("truncated stream should be protocol_error, got status=%s category=%s message=%q", res.Status, res.FailureCategory, res.Message)
	}
	if !strings.Contains(res.Message, "terminal event") {
		t.Fatalf("truncated stream should explain missing terminal event, got %q", res.Message)
	}
}

func TestRunCheckForModel_OpenAIResponses_StreamMustBeBoolean(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.6-luna", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride:     map[string]any{"stream": "true"},
	})

	if res.Status != MonitorStatusError || res.FailureCategory != MonitorFailureConfigError {
		t.Fatalf("non-boolean stream should fail locally, got status=%s category=%s message=%q", res.Status, res.FailureCategory, res.Message)
	}
	if h.callCount != 0 {
		t.Fatalf("invalid stream setting must fail before HTTP request, got calls=%d", h.callCount)
	}
}

func TestPostRawJSON_RejectsOversizedResponse(t *testing.T) {
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", monitorResponseMaxBytes+1)))
	}))
	defer srv.Close()

	_, status, _, err := postRawJSON(context.Background(), srv.URL, []byte(`{}`), nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized response should be rejected, status=%d err=%v", status, err)
	}
}

func TestRunCheckForModel_OpenAIResponses_SkipsLeadingReasoningItem(t *testing.T) {
	h := &openAICaptureHandler{responsesLeadingReasoning: true}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-5.5", &CheckOptions{
		APIMode: MonitorAPIModeResponses,
	})

	if res.Status != MonitorStatusOperational {
		t.Fatalf("responses request should find text after leading reasoning item, got status=%s message=%q", res.Status, res.Message)
	}
	if h.lastPath != providerOpenAIResponsesPath {
		t.Fatalf("expected responses path %q, got %q", providerOpenAIResponsesPath, h.lastPath)
	}
}

func TestRunCheckForModel_AnthropicSkipsLeadingNonTextBlocks(t *testing.T) {
	challenge := generateChallenge()
	h := &captureHandler{answerChallenge: true, leadingNonTextBlocks: true}
	endpoint := setupFakeAnthropic(t, h)

	text := extractAnthropicMessagesText([]byte(`{"content":[{"type":"thinking","thinking":"internal"},{"type":"tool_use","name":"noop"},{"type":"text","text":"` + challenge.Expected + `"}]}`))
	if text != challenge.Expected {
		t.Fatalf("anthropic extractor should find text after leading non-text blocks, got %q", text)
	}

	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("anthropic request should pass when text block is not first, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_OpenAIResponsesReplaceMissingInstructionsFailsLocally(t *testing.T) {
	h := &openAICaptureHandler{}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", &CheckOptions{
		APIMode:          MonitorAPIModeResponses,
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride: map[string]any{
			"model": "gpt-test",
			"input": "hello",
		},
	})

	if res.Status != MonitorStatusError {
		t.Fatalf("invalid responses replace body should fail locally as error, got status=%s", res.Status)
	}
	if res.FailureCategory != MonitorFailureConfigError {
		t.Fatalf("invalid responses replace body should be config_error, got %s", res.FailureCategory)
	}
	if res.Attempts != 1 {
		t.Fatalf("local validation should not retry, got attempts=%d", res.Attempts)
	}
	if !strings.Contains(res.Message, "instructions and input are required") {
		t.Errorf("expected local validation message about instructions/input, got %q", res.Message)
	}
	if h.lastPath != "" {
		t.Errorf("invalid replace body should fail before HTTP request, got path %q", h.lastPath)
	}
}

func TestRunCheckForModel_MergeMode_UserFieldsWinButDenyListProtects(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"system":     "You are Claude Code...",
			"max_tokens": float64(999),   // 应该覆盖默认 50
			"model":      "hacked-model", // 应该被黑名单挡住，保留原 model
			"messages":   []any{},        // 同上，被挡
		},
		ExtraHeaders: map[string]string{
			"User-Agent":     "claude-cli/1.0",
			"Content-Length": "999", // 黑名单
			"x-custom":       "ok",
		},
	}
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if h.lastBody["system"] != "You are Claude Code..." {
		t.Errorf("merge mode should inject system, got %v", h.lastBody["system"])
	}
	// max_tokens 覆盖生效
	if mt, ok := h.lastBody["max_tokens"].(float64); !ok || mt != 999 {
		t.Errorf("merge mode should override max_tokens to 999, got %v", h.lastBody["max_tokens"])
	}
	// model 在黑名单 — 应该保留默认值
	if h.lastBody["model"] != "claude-x" {
		t.Errorf("model should be protected by deny list, got %v", h.lastBody["model"])
	}
	// messages 在黑名单 — 应该保留默认值（非空）
	msgs, _ := h.lastBody["messages"].([]any)
	if len(msgs) == 0 {
		t.Error("messages should be protected by deny list (kept default, non-empty)")
	}
	// header 合并
	if h.lastHeaders.Get("User-Agent") != "claude-cli/1.0" {
		t.Errorf("extra User-Agent should override, got %q", h.lastHeaders.Get("User-Agent"))
	}
	if h.lastHeaders.Get("x-custom") != "ok" {
		t.Errorf("extra custom header should be present, got %q", h.lastHeaders.Get("x-custom"))
	}
	// Content-Length 黑名单：会被 net/http 自动重算，但不应由用户的 "999" 决定。
	// 我们无法直接断言丢弃（http.Client 总会填上），只断言请求成功即可。
}

func TestRunCheckForModel_ReplaceMode_FullBodyUsedAndChallengeSkipped(t *testing.T) {
	// replace 模式下我们的 body 完全自定义，challenge 数学题不会出现在请求里，
	// 上游也不会回正确答案 — 但只要 2xx + 响应文本非空，就算 operational
	h := &captureHandler{respondText: "any non-empty text"}
	endpoint := setupFakeAnthropic(t, h)

	userBody := map[string]any{
		"model":      "user-forced-model",
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
		"max_tokens": float64(10),
		"system":     "You are someone else",
	}
	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     userBody,
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	// 请求 body = 用户提供的原样
	if h.lastBody["model"] != "user-forced-model" {
		t.Errorf("replace mode should use user's model, got %v", h.lastBody["model"])
	}
	if h.lastBody["system"] != "You are someone else" {
		t.Errorf("replace mode should use user's system, got %v", h.lastBody["system"])
	}
	// challenge 虽然没命中，但由于 replace 模式跳过 challenge 校验 + 响应非空 → operational
	if res.Status != MonitorStatusOperational {
		t.Errorf("replace mode with 2xx + non-empty text should be operational, got status=%s message=%q",
			res.Status, res.Message)
	}
}

func TestRunCheckForModel_ReplaceMode_EmptyResponseIsFailed(t *testing.T) {
	h := &captureHandler{respondText: ""} // 上游 200 但 content[0].text 为空
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if res.Status != MonitorStatusFailed {
		t.Errorf("replace mode with empty text should be failed, got status=%s", res.Status)
	}
	if !strings.Contains(res.Message, "replace-mode") {
		t.Errorf("failure message should hint replace-mode, got %q", res.Message)
	}
	if res.FailureCategory != MonitorFailureEmptyResponse {
		t.Errorf("empty response should be categorized, got %s", res.FailureCategory)
	}
}

func TestRunCheckForModel_RetriesUpstreamErrorThenSucceeds(t *testing.T) {
	h := &openAICaptureHandler{statusSequence: []int{http.StatusBadGateway, http.StatusOK}}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("second successful attempt should be operational, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 2 || h.callCount != 2 {
		t.Fatalf("expected one retry, attempts=%d calls=%d", res.Attempts, h.callCount)
	}
	if res.FailureCategory != MonitorFailureNone {
		t.Fatalf("successful retry should clear failure category, got %s", res.FailureCategory)
	}
	if res.HTTPStatus == nil || *res.HTTPStatus != http.StatusOK {
		t.Fatalf("final HTTP status should be 200, got %v", res.HTTPStatus)
	}
	if res.RequestPath != providerOpenAIPath {
		t.Fatalf("request path should be tracked, got %q", res.RequestPath)
	}
}

func TestRunCheckForModel_RetriesEmptyResponseThenSucceeds(t *testing.T) {
	h := &openAICaptureHandler{emptyResponseUntil: 1}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("second non-empty response should be operational, got status=%s message=%q", res.Status, res.Message)
	}
	if res.Attempts != 2 || h.callCount != 2 {
		t.Fatalf("expected empty 2xx response to retry once, attempts=%d calls=%d", res.Attempts, h.callCount)
	}
}

func TestRunCheckForModel_TwoUpstreamErrorsRemainError(t *testing.T) {
	h := &openAICaptureHandler{statusSequence: []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable}}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("two upstream failures should remain error, got %s", res.Status)
	}
	if res.Attempts != 2 || h.callCount != 2 {
		t.Fatalf("expected one retry, attempts=%d calls=%d", res.Attempts, h.callCount)
	}
	if res.FailureCategory != MonitorFailureUpstreamError {
		t.Fatalf("expected upstream_error, got %s", res.FailureCategory)
	}
}

func TestRunCheckForModel_UnsupportedParameterIsConfigError(t *testing.T) {
	h := &openAICaptureHandler{
		status:    http.StatusBadRequest,
		errorBody: `{"error":{"message":"Unsupported parameter: max_output_tokens"}}`,
	}
	endpoint := setupFakeOpenAI(t, h)

	res := runCheckForModel(context.Background(), MonitorProviderOpenAI, endpoint, "sk-openai", "gpt-test", nil)

	if res.Status != MonitorStatusError {
		t.Fatalf("bad request should be error, got %s", res.Status)
	}
	if res.Attempts != 1 || h.callCount != 1 {
		t.Fatalf("config-like 400 should not retry, attempts=%d calls=%d", res.Attempts, h.callCount)
	}
	if res.FailureCategory != MonitorFailureConfigError {
		t.Fatalf("unsupported parameter should be config_error, got %s", res.FailureCategory)
	}
}

func TestExtractAnthropicMonitorText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "text block after thinking",
			body: `{"content":[{"type":"thinking","thinking":""},{"type":"text","text":"2"}]}`,
			want: "2",
		},
		{
			name: "single text block",
			body: `{"content":[{"type":"text","text":"2"}]}`,
			want: "2",
		},
		{
			name: "thinking only",
			body: `{"content":[{"type":"thinking","thinking":""}]}`,
			want: "",
		},
		{
			name: "multiple text blocks",
			body: `{"content":[{"type":"text","text":"answer"},{"type":"tool_use","name":"x"},{"type":"text","text":"2"}]}`,
			want: "answer\n2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAnthropicMonitorText([]byte(tt.body))
			if got != tt.want {
				t.Fatalf("extractAnthropicMonitorText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateChallenge_AnthropicTextAfterThinking(t *testing.T) {
	body := []byte(`{"content":[{"type":"thinking","thinking":""},{"type":"text","text":"答案是 2"}]}`)
	respText := extractAnthropicMonitorText(body)

	if !validateChallenge(respText, "2") {
		t.Fatalf("validateChallenge(%q, %q) = false, want true", respText, "2")
	}
}
