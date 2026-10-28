package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type DeepSeekTool string

const (
	DeepSeekToolSearch DeepSeekTool = "search"
	DeepSeekToolFetch  DeepSeekTool = "fetch"
)

// ForwardDeepSeekTool forwards one of DeepSeek's independent Search/Fetch APIs.
// It preserves the upstream JSON response and marks only semantically successful
// calls as billable; empty results remain HTTP 200 but are not charged.
func (s *OpenAIGatewayService) ForwardDeepSeekTool(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	tool DeepSeekTool,
	body []byte,
) (*OpenAIForwardResult, error) {
	if s == nil || c == nil || account == nil {
		return nil, fmt.Errorf("service, context, and account are required")
	}
	if account.Platform != PlatformDeepseek || account.Type != AccountTypeAPIKey || !account.IsMultiProtocolAPIKey() {
		return nil, fmt.Errorf("DeepSeek Search/Fetch requires a DeepSeek API key account")
	}
	var endpoint string
	switch tool {
	case DeepSeekToolSearch:
		endpoint = "/v1/tools/search"
	case DeepSeekToolFetch:
		endpoint = "/v1/tools/fetch"
	default:
		return nil, fmt.Errorf("unsupported DeepSeek tool")
	}

	apiKey := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key is missing")
	}
	baseURL, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return nil, err
	}
	upstreamURL := buildOpenAIEndpointURL(baseURL, endpoint)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build DeepSeek tool request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	SetActualOpenAIUpstreamEndpoint(c, endpoint)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	startedAt := time.Now()
	response, err := s.doOpenAIUpstream(request, proxyURL, account)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(startedAt).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("DeepSeek tool upstream request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	responseBody, err := ReadUpstreamResponseBody(response.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, fmt.Errorf("read DeepSeek tool response: %w", err)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden ||
		response.StatusCode == http.StatusNotFound ||
		response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
		return nil, &UpstreamFailoverError{StatusCode: response.StatusCode, ResponseBody: responseBody}
	}

	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), response.Header, s.responseHeaderFilter)
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(response.StatusCode, contentType, responseBody)
	if !deepSeekToolResponseBillable(tool, response.StatusCode, responseBody) {
		return nil, nil
	}
	model := "deepseek-tool-" + string(tool)
	return &OpenAIForwardResult{
		RequestID:       strings.TrimSpace(response.Header.Get("x-request-id")),
		UpstreamHeaders: response.Header,
		Model:           model,
		UpstreamModel:   model,
		Duration:        time.Since(startedAt),
		WebSearchCalls:  1,
	}, nil
}

func deepSeekToolResponseBillable(tool DeepSeekTool, status int, body []byte) bool {
	if status != http.StatusOK || !gjson.ValidBytes(body) {
		return false
	}
	switch tool {
	case DeepSeekToolSearch:
		results := gjson.GetBytes(body, "results")
		return results.IsArray() && len(results.Array()) > 0
	case DeepSeekToolFetch:
		content := gjson.GetBytes(body, "content")
		return content.Type == gjson.String && strings.TrimSpace(content.String()) != ""
	default:
		return false
	}
}
