package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
)

const AccountTestModePelican = "pelican"

const pelicanTestPrompt = "Generate a self-contained offline HTML document showing a looping 2D animation of a pelican riding a bicycle. Embed the SVG in the HTML and use inline CSS, SVG animation, or JavaScript as needed. Do not load external resources. Reply with the complete HTML only."

// TestPelican observes a text model's offline HTML animation without changing account scheduling state.
func (s *AccountTestService) TestPelican(c *gin.Context, accountID int64, modelID string) error {
	return s.testTextChallenge(c, accountID, modelID, AccountTestModePelican, pelicanTestPrompt)
}

func (s *AccountTestService) TestCandy(c *gin.Context, accountID int64, modelID string) error {
	return s.testTextChallenge(c, accountID, modelID, AccountTestModeCandy, candyTestPrompt)
}

func (s *AccountTestService) testTextChallenge(c *gin.Context, accountID int64, modelID, kind, prompt string) error {
	challengeName := "pelican"
	if kind == AccountTestModeCandy {
		challengeName = "candy"
	}
	ctx := c.Request.Context()
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Account not found")
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return s.sendErrorAndEnd(c, "Select a text model for the "+challengeName+" test")
	}
	if isImageGenerationModel(modelID) || account.hasImageModelLabel(modelID) || isOpenAIImageModel(modelID) {
		return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test requires a text model")
	}
	if s.httpUpstream == nil {
		return s.sendErrorAndEnd(c, "HTTP upstream not configured")
	}

	var req *http.Request
	var stream string
	switch account.Platform {
	case PlatformGemini:
		mappedModel := modelID
		if account.Type == AccountTypeAPIKey || account.Type == AccountTypeServiceAccount {
			mappedModel = account.GetMappedModel(modelID)
		}
		if isImageGenerationModel(mappedModel) || account.hasImageModelLabel(mappedModel) {
			return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test requires a text model")
		}
		payload := createGeminiTestPayloadForMode(prompt, false)
		if kind == AccountTestModeCandy {
			var payloadMap map[string]any
			if err := json.Unmarshal(payload, &payloadMap); err != nil {
				return s.sendErrorAndEnd(c, "Failed to build candy test request")
			}
			payloadMap["generationConfig"] = map[string]any{"maxOutputTokens": 512}
			payload, _ = json.Marshal(payloadMap)
		}
		switch account.Type {
		case AccountTypeAPIKey:
			req, err = s.buildGeminiAPIKeyRequest(ctx, account, mappedModel, payload)
		case AccountTypeOAuth:
			req, err = s.buildGeminiOAuthRequest(ctx, account, mappedModel, payload)
		case AccountTypeServiceAccount:
			req, err = s.buildGeminiServiceAccountRequest(ctx, account, mappedModel, payload)
		default:
			return s.sendErrorAndEnd(c, "Unsupported Gemini account type for "+challengeName+" test")
		}
		stream = "gemini"
	case PlatformOpenAI:
		if account.Type != AccountTypeAPIKey {
			return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test currently supports OpenAI API key accounts")
		}
		key := account.GetOpenAIProtocolAPIKey()
		if key == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		baseURL, err = s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, "Invalid upstream base URL")
		}
		mappedModel := account.GetMappedModel(modelID)
		if isImageGenerationModel(mappedModel) || account.hasImageModelLabel(mappedModel) || isOpenAIImageModel(mappedModel) {
			return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test requires a text model")
		}
		var payload map[string]any
		var apiURL string
		if openai_compat.ShouldUseResponsesAPI(account.Extra) {
			payload = createOpenAITestPayload(mappedModel, false)
			payload["input"] = prompt
			if kind == AccountTestModeCandy {
				payload["max_output_tokens"] = 512
			}
			apiURL = buildOpenAIResponsesURLForPlatform(account.Platform, baseURL)
			stream = "responses"
		} else {
			payload = createOpenAIChatCompletionsTestPayload(mappedModel, prompt)
			if kind == AccountTestModeCandy {
				payload["max_tokens"] = 512
			}
			apiURL = buildOpenAIChatCompletionsURL(baseURL)
			stream = "chat"
		}
		body, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
		if err == nil {
			req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Authorization", "Bearer "+key)
			account.ApplyHeaderOverrides(req.Header)
		}
	case PlatformAnthropic:
		if account.Type != AccountTypeAPIKey {
			return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test currently supports Claude API key accounts")
		}
		key := account.GetCredential("api_key")
		if key == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
		baseURL := account.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		baseURL, err = s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, "Invalid upstream base URL")
		}
		mappedModel := account.GetMappedModel(modelID)
		if isImageGenerationModel(mappedModel) || account.hasImageModelLabel(mappedModel) {
			return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test requires a text model")
		}
		payload := map[string]any{
			"model": mappedModel, "messages": []map[string]string{{"role": "user", "content": prompt}},
			"max_tokens": 4096, "stream": true,
		}
		if kind == AccountTestModeCandy {
			payload["max_tokens"] = 512
		}
		body, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/v1/messages?beta=true", bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("anthropic-version", "2023-06-01")
			setAnthropicAPIKeyAuthHeader(req.Header, account, key, account.GetBaseURL())
			account.ApplyHeaderOverrides(req.Header)
		}
		stream = "claude"
	default:
		return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" test supports Gemini, OpenAI and Claude accounts")
	}
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to build "+challengeName+" test request")
	}
	req.Header.Set("X-A6API-Self-Test-Kind", kind)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})
	s.sendEvent(c, TestEvent{Type: "technical_details", Data: pelicanRequestDetails(req)})

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return s.sendErrorAndEnd(c, strings.ToUpper(challengeName[:1])+challengeName[1:]+" upstream request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return s.sendErrorAndEnd(c, fmt.Sprintf("%s upstream returned HTTP %d", strings.ToUpper(challengeName[:1])+challengeName[1:], resp.StatusCode))
	}
	if kind == AccountTestModeCandy {
		return s.processCandyStream(c, stream, resp.Body)
	}
	switch stream {
	case "gemini":
		return s.processGeminiStream(c, resp.Body)
	case "responses":
		return s.processOpenAIStream(c, resp.Body)
	case "chat":
		return s.processOpenAIChatCompletionsStream(c, resp.Body)
	default:
		return s.processClaudeStream(c, resp.Body)
	}
}

func pelicanRequestDetails(req *http.Request) map[string]any {
	var body any
	if req.GetBody != nil {
		if reader, err := req.GetBody(); err == nil {
			defer func() { _ = reader.Close() }()
			_ = json.NewDecoder(io.LimitReader(reader, 64<<10)).Decode(&body)
		}
	}
	headers := map[string]string{
		"Content-Type":           req.Header.Get("Content-Type"),
		"X-A6API-Self-Test-Kind": req.Header.Get("X-A6API-Self-Test-Kind"),
	}
	if req.Header.Get("Authorization") != "" {
		headers["Authorization"] = "Bearer ***"
	}
	if req.Header.Get("Accept") != "" {
		headers["Accept"] = req.Header.Get("Accept")
	}
	if req.Header.Get("anthropic-version") != "" {
		headers["anthropic-version"] = req.Header.Get("anthropic-version")
	}
	if req.Header.Get("x-api-key") != "" {
		headers["x-api-key"] = "***"
	}
	if req.Header.Get("x-goog-api-key") != "" {
		headers["x-goog-api-key"] = "***"
	}
	return map[string]any{
		"method":  req.Method,
		"path":    req.URL.RequestURI(),
		"headers": headers,
		"body":    body,
	}
}
