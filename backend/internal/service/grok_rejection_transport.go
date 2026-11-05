package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const rejectionStateKey = "grok_rejection_attempt"

type grokRejectionAttempt struct {
	replaced atomic.Bool
	failed   atomic.Bool
}

func GrokNormalBillingReplaced(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, _ := c.Get(rejectionStateKey)
	s, _ := v.(*grokRejectionAttempt)
	return s != nil && (s.replaced.Load() || s.failed.Load())
}
func stampGrokRejection(c *gin.Context, result *OpenAIForwardResult) {
	if result != nil {
		result.GrokNormalBillingReplaced = GrokNormalBillingReplaced(c)
	}
}

// Called for each actual upstream attempt, including retries. No user identity
// is taken from the response, request payload or client-supplied headers.
func (s *OpenAIGatewayService) prepareGrokRejection(request *http.Request, account *Account, c *gin.Context) (func(*http.Response) error, error) {
	if c == nil {
		return nil, nil
	}
	c.Set(rejectionStateKey, (*grokRejectionAttempt)(nil))
	value, _ := c.Get("api_key")
	key, _ := value.(*APIKey)
	if key == nil || key.User == nil || key.UserID != key.User.ID || key.GroupID == nil || s.channelService == nil {
		return nil, nil
	}
	// Video polling can pin a multi-group key to the task's original group.
	// That resolved key is server-owned ops context, never proof of auth itself.
	// Require the already authenticated identity and configured group membership.
	if value, ok := c.Get("ops_fallback_api_key"); ok {
		if selected, ok := value.(*APIKey); ok && selected != nil && selected.ID == key.ID && selected.UserID == key.UserID && selected.User == key.User && selected.GroupID != nil {
			for _, groupID := range key.ConfiguredGroupIDs() {
				if groupID == *selected.GroupID {
					key = selected
					break
				}
			}
		}
	}
	channel, err := s.channelService.GetChannelForGroup(request.Context(), *key.GroupID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, nil
	}
	cfg, err := ParseGrokRejectionConfig(channel.FeaturesConfig)
	if err != nil {
		return nil, err
	}
	if !cfg.Trusts(request.URL.String()) {
		return nil, nil
	}
	repo, ok := s.usageBillingRepo.(GrokRejectionBillingRepository)
	if !ok {
		return nil, errors.New("Grok rejection billing repository unavailable")
	}
	taskID := ""
	state := &grokRejectionAttempt{}
	if request.Method == http.MethodGet {
		if _, suffix, found := strings.Cut(request.URL.Path, "/videos/"); found {
			taskID = strings.Split(suffix, "/")[0]
		}
	}
	if taskID != "" {
		owns, e := repo.OwnsGrokRejectionTask(request.Context(), cfg.SourceID, taskID, key.UserID)
		if e != nil {
			return nil, e
		}
		if !owns {
			return nil, errors.New("video task not found for authenticated user")
		}
		replaced, e := repo.GrokTaskNormalBillingReplaced(request.Context(), cfg.SourceID, taskID, key.UserID)
		if e != nil {
			return nil, e
		}
		state.replaced.Store(replaced)
	}
	if request.Method == http.MethodPost && key.User.Balance <= 0 {
		return nil, errors.New("wallet balance is required for upstream rejection fees, including subscriptions")
	}
	c.Set(rejectionStateKey, state)
	model := "grok"
	if v := c.GetString(OpsUpstreamModelKey); v != "" {
		model = v
	}
	if request.GetBody != nil && strings.Contains(request.Header.Get("Content-Type"), "application/json") {
		if body, e := request.GetBody(); e == nil {
			var payload struct {
				Model string `json:"model"`
			}
			e = json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&payload)
			_ = body.Close()
			if e == nil && payload.Model != "" {
				model = payload.Model
			}
		}
	}
	// Snapshot only accounting metadata; never retain request bodies or credentials.
	log := UsageLog{UserID: key.UserID, APIKeyID: key.ID, AccountID: account.ID, GroupID: key.GroupID, ChannelID: &channel.ID, Model: model, RequestedModel: model, CreatedAt: time.Now(), RateMultiplier: 1, RequestType: RequestTypeSync}
	inbound, upstream := c.Request.URL.Path, request.URL.Path
	log.InboundEndpoint = &inbound
	log.UpstreamEndpoint = &upstream
	localID := uuid.NewString()
	persist := func(receipt GrokRejectionReceipt) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		command := &GrokRejectionCommand{SourceID: cfg.SourceID, AccountType: account.Type, LocalRequestID: localID, Receipt: receipt, NormalBillingReplaced: state.replaced.Load(), Usage: log}
		if err := repo.EnqueueGrokRejection(ctx, command); err != nil {
			state.failed.Store(true)
			// Minimal financial evidence permits reconciliation if the database
			// itself is unavailable. Never log generated content or credentials.
			slog.Error("grok_rejection_inbox_failed", "user_id", key.UserID, "api_key_id", key.ID, "account_id", account.ID,
				"source_id", cfg.SourceID, "billing_request_id", receipt.BillingRequestID, "billing_status", receipt.BillingStatus,
				"upstream_amount", receipt.UpstreamAmount, "user_amount", receipt.UserAmount, "local_request_id", localID, "error", err)
			return err
		}
		if !receipt.Chargeable() {
			return nil
		}
		result, err := repo.SettleGrokRejection(ctx, command.EventKey())
		if err != nil {
			slog.Error("grok_rejection_settlement_pending", "event_key", command.EventKey(), "error", err)
			return nil
		} // durable recovery owns retry
		if result != nil && result.Applied {
			s.invalidateGrokRejectionCaches(ctx, key.UserID, key.ID)
		}
		return nil
	}
	return func(resp *http.Response) error {
		if resp == nil || resp.Body == nil {
			return nil
		}
		if resp.Request != nil && !cfg.Trusts(resp.Request.URL.String()) {
			return errors.New("untrusted redirect for Grok rejection billing")
		}
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "application/octet-stream") {
			return nil
		}
		streaming := strings.Contains(ct, "text/event-stream")
		log.Stream = streaming
		log.RequestType = RequestTypeFromLegacy(streaming, false)
		var process func([]byte, int) error
		process = func(data []byte, depth int) error {
			if depth > 2 {
				return persist(GrokRejectionReceipt{ReviewReason: "invalid_billing_envelope_depth"})
			}
			if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
				return nil
			}
			var envelope map[string]json.RawMessage
			if json.Unmarshal(data, &envelope) != nil {
				return persist(GrokRejectionReceipt{ReviewReason: "malformed_upstream_response"})
			}
			receipts, replaced, err := ParseGrokRejectionReceipts(data, cfg)
			if replaced {
				state.replaced.Store(true)
				if taskID != "" {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					e := repo.ReplaceGrokTaskNormalBilling(ctx, cfg.SourceID, taskID, key.UserID)
					cancel()
					if e != nil {
						return e
					}
				}
			}
			if err != nil {
				return persist(GrokRejectionReceipt{ReviewReason: "invalid_billing_envelope"})
			}
			for _, r := range receipts {
				if err := persist(r); err != nil {
					return err
				}
			}
			// Known protocol envelopes only; generated text is never traversed.
			for _, name := range []string{"response", "error"} {
				raw := envelope[name]
				if len(raw) > 0 && (bytes.Contains(raw, []byte(`"grok_rejection_billing"`)) || bytes.Contains(raw, []byte(`"normal_billing"`))) {
					var child map[string]json.RawMessage
					if json.Unmarshal(raw, &child) == nil && (child["grok_rejection_billing"] != nil || child["normal_billing"] != nil) {
						if err := process(raw, depth+1); err != nil {
							return err
						}
					}
				}
			}
			if len(receipts) == 0 {
				var e struct {
					Code string `json:"code"`
					Type string `json:"type"`
				}
				_ = json.Unmarshal(envelope["error"], &e)
				if replaced || envelope["grok_rejection_billing"] != nil || e.Code == "content_policy_violation" || e.Code == "content_policy_rejection" || e.Type == "content_policy_violation" {
					if err := persist(GrokRejectionReceipt{ReviewReason: "missing_receipt"}); err != nil {
						return err
					}
				}
			}
			if request.Method == http.MethodPost && strings.Contains(request.URL.Path, "/videos") && resp.StatusCode < 300 {
				id := ""
				_ = json.Unmarshal(envelope["request_id"], &id)
				if id == "" {
					_ = json.Unmarshal(envelope["id"], &id)
				}
				if id != "" {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := repo.BindGrokRejectionTask(ctx, cfg.SourceID, id, key.UserID, key.ID); err != nil {
						return err
					}
					if state.replaced.Load() {
						if err := repo.ReplaceGrokTaskNormalBilling(ctx, cfg.SourceID, id, key.UserID); err != nil {
							return err
						}
					}
				}
			}
			return nil
		}
		limit := resolveUpstreamResponseReadLimit(s.cfg)
		if streaming {
			limit = int64(defaultMaxLineSize)
			if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
				limit = int64(s.cfg.Gateway.MaxLineSize)
			}
		}
		resp.Body = &grokRejectionBody{source: resp.Body, stream: streaming, limit: limit, process: func(data []byte) error { return process(data, 0) }, onIncomplete: func() error { return persist(GrokRejectionReceipt{ReviewReason: "incomplete_response"}) }}
		return nil
	}, nil
}

// Observe bytes before downstream filtering/writing, preserving the exact wire
// payload and backpressure. Memory is bounded independently of response length.
type grokRejectionBody struct {
	readMu       sync.Mutex
	closeOnce    sync.Once
	closeErr     error
	source       io.ReadCloser
	stream       bool
	limit        int64
	buffer       []byte
	event        []byte
	done         bool
	terminal     bool
	process      func([]byte) error
	onIncomplete func() error
}

func (b *grokRejectionBody) Read(p []byte) (int, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	n, err := b.source.Read(p)
	if !b.done && (n > 0 || (b.stream && err == io.EOF)) {
		b.buffer = append(b.buffer, p[:n]...)
		limit := b.limit
		if limit <= 0 {
			limit = defaultUpstreamResponseReadMaxBytes
			if b.stream {
				limit = defaultMaxLineSize
			}
		}
		if int64(len(b.buffer)+len(b.event)) > limit {
			b.done = true
			_ = b.onIncomplete()
			return 0, errors.New("Grok billing response exceeds observation limit")
		}
		if b.stream {
			for {
				advance, rawLine, _ := scanSSELinesPreservingEndings(b.buffer, err == io.EOF)
				if advance == 0 {
					break
				}
				line := trimSSELineEnding(rawLine)
				b.buffer = b.buffer[advance:]
				if len(line) == 0 {
					if len(b.event) > 0 {
						data := bytes.TrimSuffix(b.event, []byte{'\n'})
						if e := b.process(data); e != nil {
							b.done = true
							return 0, e
						}
						var event struct {
							Type string `json:"type"`
						}
						_ = json.Unmarshal(data, &event)
						if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) || event.Type == "response.completed" || event.Type == "response.failed" || event.Type == "response.incomplete" {
							b.terminal = true
						}
						b.event = nil
					}
				} else if bytes.HasPrefix(line, []byte("data:")) {
					data := line[5:]
					data = bytes.TrimPrefix(data, []byte{' '})
					b.event = append(b.event, data...)
					b.event = append(b.event, '\n')
				}
			}
		} else if trimmed := bytes.TrimSpace(p[:n]); len(trimmed) > 0 && trimmed[len(trimmed)-1] == '}' && json.Valid(b.buffer) {
			b.done = true
			if e := b.process(b.buffer); e != nil {
				return 0, e
			}
			b.buffer = nil
		}
	}
	if err != nil && !b.done {
		b.done = true
		if b.stream && b.terminal && len(b.buffer) == 0 && len(b.event) == 0 {
			return n, err
		}
		if e := b.onIncomplete(); e != nil {
			return 0, e
		}
	}
	return n, err
}
func (b *grokRejectionBody) Close() error {
	// Close the socket first to unblock an in-flight Read, then wait for the
	// bytes it already received to be accounted for before marking incomplete.
	b.closeOnce.Do(func() { b.closeErr = b.source.Close() })
	b.readMu.Lock()
	defer b.readMu.Unlock()
	if !b.done && (!b.terminal || len(b.buffer) > 0 || len(b.event) > 0) {
		b.done = true
		_ = b.onIncomplete()
	}
	return b.closeErr
}

func (s *OpenAIGatewayService) invalidateGrokRejectionCaches(ctx context.Context, user, key int64) {
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, user)
		_ = s.billingCacheService.InvalidateAPIKeyRateLimit(ctx, key)
	}
	// Key quota is enforced by the auth cache too; invalidate all keys for owner.
	if s.channelService != nil && s.channelService.authCacheInvalidator != nil {
		s.channelService.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, user)
	}
}

type grokRejectionRecovery struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (s *OpenAIGatewayService) startGrokRejectionRecovery() {
	repo, ok := s.usageBillingRepo.(GrokRejectionBillingRepository)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.rejectionRecovery = &grokRejectionRecovery{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.rejectionRecovery.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			s.recoverGrokRejections(ctx, repo)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (s *OpenAIGatewayService) recoverGrokRejections(ctx context.Context, repo GrokRejectionBillingRepository) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	keys, err := repo.PendingGrokRejections(ctx, 50)
	if err != nil {
		slog.Error("grok_rejection_recovery_failed", "error", err)
		return
	}
	for _, key := range keys {
		result, e := repo.SettleGrokRejection(ctx, key)
		if e != nil {
			slog.Error("grok_rejection_recovery_pending", "event_key", key, "error", e)
		} else if result != nil && result.Applied {
			s.invalidateGrokRejectionCaches(ctx, result.RejectionUserID, result.RejectionAPIKeyID)
		}
	}
}
func (s *OpenAIGatewayService) StopGrokRejectionRecovery() {
	if r := s.rejectionRecovery; r != nil {
		r.once.Do(r.cancel)
		<-r.done
	}
}
