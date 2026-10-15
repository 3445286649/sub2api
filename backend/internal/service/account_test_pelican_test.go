//go:build unit

package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

func TestPelicanRequestDetailsRedactsCredentials(t *testing.T) {
	payload := map[string]any{"model": "test-model", "messages": []map[string]string{{"role": "user", "content": pelicanTestPrompt}}, "stream": true}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("X-A6API-Self-Test-Kind", "pelican")
	details := pelicanRequestDetails(req)
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("secret-token")) {
		t.Fatal("request details disclosed the upstream credential")
	}
	if details["path"] != "/v1/chat/completions" {
		t.Fatalf("unexpected path: %v", details["path"])
	}
	gotBody := details["body"].(map[string]any)
	if gotBody["model"] != "test-model" {
		t.Fatalf("unexpected request body: %v", gotBody)
	}
}

func TestPelicanUsesConfiguredProtocolWithoutChangingAccountState(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		response string
		path     string
	}{
		{
			name: "Gemini native", account: &Account{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret"}},
			response: "data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[{\"text\":\"<svg></svg>\"}]}}]}\n\n",
			path:     "/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		},
		{
			name: "OpenAI Chat Completions", account: &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret"}, Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: false}},
			response: "data: {\"choices\":[{\"delta\":{\"content\":\"<svg></svg>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n",
			path:     "/v1/chat/completions",
		},
		{
			name: "OpenAI Responses", account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret"}, Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: true}},
			response: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<svg></svg>\"}\n\ndata: {\"type\":\"response.completed\"}\n\n",
			path:     "/v1/responses",
		},
		{
			name: "Claude native", account: &Account{ID: 4, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "secret"}},
			response: "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"<svg></svg>\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n",
			path:     "/v1/messages",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{tt.account.ID: tt.account}}}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK, tt.response)}}
			svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
			ctx, recorder := newTestContext()
			model := "gemini-2.5-flash"
			if tt.account.Platform == PlatformOpenAI {
				model = "gpt-5.4"
			} else if tt.account.Platform == PlatformAnthropic {
				model = "claude-sonnet-4-6"
			}
			if err := svc.TestPelican(ctx, tt.account.ID, model); err != nil {
				t.Fatal(err)
			}
			if len(upstream.requests) != 1 {
				t.Fatalf("upstream request count = %d", len(upstream.requests))
			}
			req := upstream.requests[0]
			if req.URL.Path != tt.path || req.Header.Get("X-A6API-Self-Test-Kind") != "pelican" {
				t.Fatalf("unexpected upstream request: %s %s", req.URL.Path, req.Header.Get("X-A6API-Self-Test-Kind"))
			}
			body, _ := io.ReadAll(req.Body)
			if !bytes.Contains(body, []byte(pelicanTestPrompt)) {
				t.Fatalf("pelican prompt missing from %s request", tt.name)
			}
			if !strings.Contains(recorder.Body.String(), `"type":"technical_details"`) || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatal("technical details missing or secret exposed")
			}
			if repo.setErrorID != 0 || repo.rateLimitedID != 0 || repo.updatedExtra != nil {
				t.Fatal("pelican test modified account state")
			}
		})
	}
}
