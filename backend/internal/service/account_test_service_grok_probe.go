package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

const (
	GrokProbeProtocolOpenAIChatCompletions = "openai_chat_completions"
	GrokProbeProtocolAnthropicMessages     = "anthropic_messages"
	GrokProbeProtocolOpenAIResponses       = "openai_responses"

	GrokProbeStatusSupported   = "supported"
	GrokProbeStatusUnsupported = "unsupported"
	GrokProbeStatusUnknown     = "unknown"
)

const grokProtocolProbeBodyLimit = 16 * 1024

type GrokProtocolProbeResult struct {
	Protocol   string `json:"protocol"`
	Label      string `json:"label"`
	Endpoint   string `json:"endpoint"`
	Supported  bool   `json:"supported"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code,omitempty"`
	Message    string `json:"message,omitempty"`
}

type GrokProtocolProbeResponse struct {
	Results             []GrokProtocolProbeResult `json:"results"`
	RecommendedProtocol string                    `json:"recommended_protocol,omitempty"`
}

func (s *AccountTestService) ProbeGrokProtocols(ctx context.Context, account *Account, modelID string, prompt string) (*GrokProtocolProbeResponse, error) {
	if s == nil {
		return nil, errors.New("account test service is not configured")
	}
	if account == nil {
		return nil, errors.New("account is required")
	}
	if account.Platform != PlatformGrok || account.Type != AccountTypeAPIKey {
		return nil, errors.New("Grok protocol probe only supports Grok API key accounts")
	}
	if s.httpUpstream == nil {
		return nil, errors.New("upstream HTTP client is not configured")
	}

	baseURL, err := s.validateUpstreamBaseURL(account.GetGrokBaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid Grok base URL: %w", err)
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, errors.New("Grok API key is required")
	}
	modelID = resolveGrokProtocolProbeModel(account, modelID)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "hi"
	}

	probes := []struct {
		protocol string
		label    string
		endpoint string
		build    func(context.Context) (*http.Request, error)
	}{
		{
			protocol: GrokProbeProtocolOpenAIChatCompletions,
			label:    "OpenAI Chat Completions",
			endpoint: "/v1/chat/completions",
			build: func(ctx context.Context) (*http.Request, error) {
				payload := map[string]any{
					"model":      modelID,
					"messages":   []map[string]any{{"role": "user", "content": prompt}},
					"max_tokens": 1,
					"stream":     false,
				}
				req, err := newJSONProbeRequest(ctx, buildOpenAIChatCompletionsURL(baseURL), payload)
				if err != nil {
					return nil, err
				}
				req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
				req.Header.Set("Authorization", "Bearer "+apiKey)
				return req, nil
			},
		},
		{
			protocol: GrokProbeProtocolAnthropicMessages,
			label:    "Anthropic Messages",
			endpoint: "/v1/messages",
			build: func(ctx context.Context) (*http.Request, error) {
				payload := map[string]any{
					"model":      modelID,
					"messages":   []map[string]any{{"role": "user", "content": prompt}},
					"max_tokens": 1,
					"stream":     false,
				}
				req, err := newJSONProbeRequest(ctx, buildV1MessagesProbeURL(baseURL), payload)
				if err != nil {
					return nil, err
				}
				req.Header.Set("anthropic-version", "2023-06-01")
				req.Header.Set("anthropic-beta", claude.DefaultBetaHeader)
				setAnthropicAPIKeyAuthHeader(req.Header, account, apiKey)
				return req, nil
			},
		},
		{
			protocol: GrokProbeProtocolOpenAIResponses,
			label:    "OpenAI Responses",
			endpoint: "/v1/responses",
			build: func(ctx context.Context) (*http.Request, error) {
				payload := map[string]any{
					"model":             modelID,
					"input":             prompt,
					"max_output_tokens": 1,
					"stream":            false,
				}
				req, err := newJSONProbeRequest(ctx, buildOpenAIResponsesURL(baseURL), payload)
				if err != nil {
					return nil, err
				}
				req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
				req.Header.Set("Authorization", "Bearer "+apiKey)
				return req, nil
			},
		},
	}

	results := make([]GrokProtocolProbeResult, 0, len(probes))
	proxyURL := upstreamModelsProxyURL(account)
	for _, probe := range probes {
		req, buildErr := probe.build(ctx)
		result := GrokProtocolProbeResult{Protocol: probe.protocol, Label: probe.label, Endpoint: probe.endpoint}
		if buildErr != nil {
			result.Status = GrokProbeStatusUnknown
			result.Message = buildErr.Error()
			results = append(results, result)
			continue
		}
		account.ApplyHeaderOverrides(req.Header)
		resp, doErr := s.doUpstreamModelsRequest(req, proxyURL, account)
		if doErr != nil {
			result.Status = GrokProbeStatusUnknown
			result.Message = doErr.Error()
			results = append(results, result)
			continue
		}
		body, readErr := readLimitedProbeBody(resp.Body)
		_ = resp.Body.Close()
		result.StatusCode = resp.StatusCode
		if readErr != nil {
			result.Status = GrokProbeStatusUnknown
			result.Message = readErr.Error()
			results = append(results, result)
			continue
		}
		result.Status, result.Supported, result.Message = classifyGrokProtocolProbeResponse(resp.StatusCode, body)
		results = append(results, result)
	}

	return &GrokProtocolProbeResponse{
		Results:             results,
		RecommendedProtocol: recommendedGrokProtocol(results),
	}, nil
}

func resolveGrokProtocolProbeModel(account *Account, requested string) string {
	modelID := strings.TrimSpace(requested)
	if modelID != "" {
		if mapped := strings.TrimSpace(account.GetMappedModel(modelID)); mapped != "" {
			return mapped
		}
		return modelID
	}
	mapping := account.GetModelMapping()
	for from, to := range mapping {
		if strings.TrimSpace(to) != "" {
			return strings.TrimSpace(to)
		}
		if strings.TrimSpace(from) != "" {
			return strings.TrimSpace(from)
		}
	}
	return "grok-4"
}

func newJSONProbeRequest(ctx context.Context, url string, payload map[string]any) (*http.Request, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sub2api-grok-probe/1.0")
	return req, nil
}

func readLimitedProbeBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(body, grokProtocolProbeBodyLimit+1))
	if err != nil {
		return nil, err
	}
	if len(b) > grokProtocolProbeBodyLimit {
		return b[:grokProtocolProbeBodyLimit], nil
	}
	return b, nil
}

func classifyGrokProtocolProbeResponse(statusCode int, body []byte) (status string, supported bool, message string) {
	msg := sanitizeGrokProbeMessage(body)
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return GrokProbeStatusSupported, true, "protocol responded successfully"
	}
	if isGrokProtocolProbeUnsupported(statusCode, msg) {
		if msg == "" {
			msg = fmt.Sprintf("upstream returned HTTP %d", statusCode)
		}
		return GrokProbeStatusUnsupported, false, msg
	}
	if msg == "" {
		msg = fmt.Sprintf("upstream returned HTTP %d", statusCode)
	}
	return GrokProbeStatusUnknown, false, msg
}

func isGrokProtocolProbeUnsupported(statusCode int, message string) bool {
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
		return true
	}
	if statusCode != http.StatusBadRequest {
		return false
	}
	lower := strings.ToLower(message)
	markers := []string{
		"route not found",
		"not found",
		"unsupported endpoint",
		"unsupported api",
		"unknown endpoint",
		"invalid endpoint",
		"unknown parameter",
		"unrecognized request argument",
		"not supported",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sanitizeGrokProbeMessage(body []byte) string {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		if extracted := extractJSONErrorMessage(parsed); extracted != "" {
			msg = extracted
		}
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > 300 {
		msg = msg[:300] + "..."
	}
	return msg
}

func extractJSONErrorMessage(value map[string]any) string {
	if errValue, ok := value["error"]; ok {
		switch errData := errValue.(type) {
		case string:
			return strings.TrimSpace(errData)
		case map[string]any:
			for _, key := range []string{"message", "type", "code"} {
				if s, ok := errData[key].(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	for _, key := range []string{"message", "detail", "type", "code"} {
		if s, ok := value[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func recommendedGrokProtocol(results []GrokProtocolProbeResult) string {
	for _, result := range results {
		if result.Protocol == GrokProbeProtocolOpenAIChatCompletions && result.Supported {
			return GrokUpstreamProtocolOpenAIChatCompletions
		}
	}
	for _, result := range results {
		if result.Protocol == GrokProbeProtocolAnthropicMessages && result.Supported {
			return GrokUpstreamProtocolAnthropicMessages
		}
	}
	return ""
}

func buildV1MessagesProbeURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(normalized, "/v1/messages") {
		return normalized + "?beta=true"
	}
	if strings.HasSuffix(normalized, "/v1") {
		return normalized + "/messages?beta=true"
	}
	return normalized + "/v1/messages?beta=true"
}
