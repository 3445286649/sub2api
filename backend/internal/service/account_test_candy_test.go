//go:build unit

package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

const candyCorrectAnswer = `{"minimum_draws":21,"round":9,"star":12,"proof":{"round_watermelon_capacity":8,"star_capacity_without_apple":10,"star_capacity_without_peach":11,"one_less_round_quota":8}}`

func candySSEData(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(data) + "\n\n"
}

func TestProcessCandyStreamScoresAllSupportedProtocols(t *testing.T) {
	tests := []struct {
		name   string
		stream string
		body   string
	}{
		{
			name:   "Gemini",
			stream: "gemini",
			body: candySSEData(t, map[string]any{"candidates": []any{map[string]any{
				"content":      map[string]any{"parts": []any{map[string]any{"text": candyCorrectAnswer}}},
				"finishReason": "STOP",
			}}}),
		},
		{
			name:   "Responses",
			stream: "responses",
			body: candySSEData(t, map[string]any{"type": "response.output_text.delta", "delta": candyCorrectAnswer}) +
				candySSEData(t, map[string]any{"type": "response.completed"}),
		},
		{
			name:   "Chat Completions",
			stream: "chat",
			body: candySSEData(t, map[string]any{"choices": []any{map[string]any{
				"delta":         map[string]any{"content": candyCorrectAnswer},
				"finish_reason": "stop",
			}}}),
		},
		{
			name:   "Claude",
			stream: "claude",
			body: candySSEData(t, map[string]any{"type": "content_block_delta", "delta": map[string]any{"text": candyCorrectAnswer}}) +
				candySSEData(t, map[string]any{"type": "message_stop"}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &AccountTestService{}
			ctx, recorder := newTestContext()
			if err := svc.processCandyStream(ctx, tt.stream, strings.NewReader(tt.body)); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(recorder.Body.String(), `"type":"test_complete"`) ||
				!strings.Contains(recorder.Body.String(), `"status":"correct"`) {
				t.Fatalf("expected a correct completion event, got: %s", recorder.Body.String())
			}
		})
	}
}

func TestScoreCandyAnswerDistinguishesIncorrectAndUnscorable(t *testing.T) {
	incorrect := scoreCandyAnswer(`{"minimum_draws":20,"round":8,"star":12,"proof":{"round_watermelon_capacity":8,"star_capacity_without_apple":10,"star_capacity_without_peach":11,"one_less_round_quota":8}}`, false)
	if incorrect.Status != "incorrect" || incorrect.ParsedAnswer == nil {
		t.Fatalf("expected an incorrect parsed answer, got %+v", incorrect)
	}
	unscorable := scoreCandyAnswer("I cannot solve this.", false)
	if unscorable.Status != "unscorable" || unscorable.ParsedAnswer != nil {
		t.Fatalf("expected an unscorable answer, got %+v", unscorable)
	}
	wrongProof := scoreCandyAnswer(`{"minimum_draws":21,"round":9,"star":12,"proof":{"round_watermelon_capacity":7,"star_capacity_without_apple":10,"star_capacity_without_peach":11,"one_less_round_quota":8}}`, false)
	if wrongProof.Status != "incorrect" {
		t.Fatalf("wrong proof was scored as %q, want incorrect", wrongProof.Status)
	}
	trailingContradiction := scoreCandyAnswer(candyCorrectAnswer+` The answer is actually 20.`, false)
	if trailingContradiction.Status != "unscorable" {
		t.Fatalf("trailing contradiction was scored as %q, want unscorable", trailingContradiction.Status)
	}
	additionalContradiction := scoreCandyAnswer(strings.TrimSuffix(candyCorrectAnswer, "}")+`,"reasoning":"the minimum is 20"}`, false)
	if additionalContradiction.Status != "unscorable" {
		t.Fatalf("additional contradictory field was scored as %q, want unscorable", additionalContradiction.Status)
	}
	duplicateAnswerField := scoreCandyAnswer(strings.Replace(candyCorrectAnswer, `"round":9`, `"round":8,"round":9`, 1), false)
	if duplicateAnswerField.Status != "unscorable" {
		t.Fatalf("duplicate answer field was scored as %q, want unscorable", duplicateAnswerField.Status)
	}
	tooLong := scoreCandyAnswer(candyCorrectAnswer, true)
	if tooLong.Status != "unscorable" {
		t.Fatalf("oversized output status = %q, want unscorable", tooLong.Status)
	}
}

func TestProcessCandyStreamHandlesFinalUnterminatedEventAndLimitsInput(t *testing.T) {
	t.Run("unterminated done event", func(t *testing.T) {
		svc := &AccountTestService{}
		ctx, recorder := newTestContext()
		body := candySSEData(t, map[string]any{"choices": []any{map[string]any{
			"delta": map[string]any{"content": candyCorrectAnswer},
		}}}) + "data: [DONE]"
		if err := svc.processCandyStream(ctx, "chat", strings.NewReader(body)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(recorder.Body.String(), `"status":"correct"`) {
			t.Fatalf("final unterminated event was not processed: %s", recorder.Body.String())
		}
	})

	t.Run("single oversized event line", func(t *testing.T) {
		svc := &AccountTestService{}
		ctx, recorder := newTestContext()
		body := "data: " + strings.Repeat("x", candyStreamLimit)
		if err := svc.processCandyStream(ctx, "chat", strings.NewReader(body)); err == nil {
			t.Fatal("expected oversized upstream stream to be rejected")
		}
		if !strings.Contains(recorder.Body.String(), "Candy upstream stream exceeded the size limit") ||
			strings.Contains(recorder.Body.String(), `"type":"test_complete"`) {
			t.Fatalf("unexpected oversized-stream output: %s", recorder.Body.String())
		}
	})
}

func TestCandyUsesAccountProtocolAndDoesNotChangeAccountState(t *testing.T) {
	account := &Account{
		ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-candy-secret"},
		Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
	}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK,
		candySSEData(t, map[string]any{"choices": []any{map[string]any{
			"delta":         map[string]any{"content": candyCorrectAnswer},
			"finish_reason": "stop",
		}}}),
	)}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	ctx, recorder := newTestContext()
	if err := svc.TestCandy(ctx, account.ID, "gpt-6-astra"); err != nil {
		t.Fatal(err)
	}
	if len(upstream.requests) != 1 {
		t.Fatalf("upstream requests = %d, want 1", len(upstream.requests))
	}
	req := upstream.requests[0]
	if req.URL.Path != "/v1/chat/completions" || req.Header.Get("X-A6API-Self-Test-Kind") != AccountTestModeCandy {
		t.Fatalf("unexpected candy upstream request: %s %s", req.URL.Path, req.Header.Get("X-A6API-Self-Test-Kind"))
	}
	requestBody, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = requestBody.Close() }()
	body, err := io.ReadAll(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload["messages"].([]any)[0].(map[string]any)["content"].(string), candyTestPrompt) || payload["max_tokens"] != float64(512) {
		t.Fatalf("unexpected candy request payload: %s", body)
	}
	if repo.setErrorID != 0 || repo.rateLimitedID != 0 || repo.updatedExtra != nil {
		t.Fatal("candy test modified account scheduling or error state")
	}
	if strings.Contains(recorder.Body.String(), "sk-candy-secret") {
		t.Fatal("candy technical details exposed the API key")
	}
	if !strings.Contains(recorder.Body.String(), `"status":"correct"`) {
		t.Fatalf("expected candy score in SSE completion: %s", recorder.Body.String())
	}
}

func TestCandyUpstreamFailureIsRedactedAndDoesNotChangeAccountState(t *testing.T) {
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-candy-secret"}}
	repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusBadGateway, `{"error":{"message":"failed using sk-candy-secret"}}`)}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream, cfg: &config.Config{}}
	ctx, recorder := newTestContext()
	if err := svc.TestCandy(ctx, account.ID, "gpt-6-astra"); err == nil {
		t.Fatal("expected an upstream error")
	}
	output := recorder.Body.String()
	if !strings.Contains(output, "Candy upstream returned HTTP 502") ||
		strings.Contains(output, "sk-candy-secret") || strings.Contains(output, "failed using") {
		t.Fatalf("unexpected error output: %s", output)
	}
	if repo.setErrorID != 0 || repo.rateLimitedID != 0 || repo.updatedExtra != nil {
		t.Fatal("failed candy test modified account state")
	}
}
