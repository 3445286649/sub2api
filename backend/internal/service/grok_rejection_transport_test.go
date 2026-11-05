package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rejectionRepoFake struct {
	UsageBillingRepository
	commands    []*GrokRejectionCommand
	settled     map[string]bool
	totals      float64
	owns        bool
	failEnqueue bool
	failSettle  bool
}

func (r *rejectionRepoFake) GrokTaskNormalBillingReplaced(context.Context, string, string, int64) (bool, error) {
	return false, nil
}
func (r *rejectionRepoFake) ReplaceGrokTaskNormalBilling(context.Context, string, string, int64) error {
	return nil
}

func (r *rejectionRepoFake) EnqueueGrokRejection(_ context.Context, c *GrokRejectionCommand) error {
	if r.failEnqueue {
		return errors.New("simulated database outage")
	}
	r.commands = append(r.commands, c)
	return nil
}
func (r *rejectionRepoFake) SettleGrokRejection(_ context.Context, key string) (*UsageBillingApplyResult, error) {
	if r.failSettle {
		return nil, errors.New("simulated transaction failure")
	}
	if r.settled == nil {
		r.settled = map[string]bool{}
	}
	if r.settled[key] {
		return &UsageBillingApplyResult{}, nil
	}
	r.settled[key] = true
	r.totals++
	return &UsageBillingApplyResult{Applied: true}, nil
}
func (r *rejectionRepoFake) PendingGrokRejections(context.Context, int) ([]string, error) {
	return nil, nil
}
func (r *rejectionRepoFake) BindGrokRejectionTask(context.Context, string, string, int64, int64) error {
	r.owns = true
	return nil
}
func (r *rejectionRepoFake) OwnsGrokRejectionTask(context.Context, string, string, int64) (bool, error) {
	return r.owns, nil
}

func rejectionTestGateway(t *testing.T) (*OpenAIGatewayService, *gin.Context, *rejectionRepoFake, *Account) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	gid := int64(3)
	c.Set("api_key", &APIKey{ID: 2, UserID: 1, User: &User{ID: 1, Balance: 10}, GroupID: &gid})
	channel := &Channel{ID: 4, Status: StatusActive, FeaturesConfig: map[string]any{"grok_rejection_billing": map[string]any{"enabled": true, "source_id": "supe", "allowed_origins": []string{"https://supe.example"}, "amount_mode": "upstream"}}}
	channels := &ChannelService{}
	channels.cache.Store(&channelCache{loadedAt: time.Now(), channelByGroupID: map[int64]*Channel{gid: channel}})
	repo := &rejectionRepoFake{}
	return &OpenAIGatewayService{channelService: channels, usageBillingRepo: repo}, c, repo, &Account{ID: 9, Platform: PlatformGrok}
}

type rejectionChunkReader struct {
	data []byte
	size int
}

func (r *rejectionChunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.size, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}
func (r *rejectionChunkReader) Close() error { return nil }

func TestGrokRejectionTransportJSONAndSSE(t *testing.T) {
	const receipt = `{"normal_billing":"replaced","grok_rejection_billing":{"charges":[{"billing_request_id":"a","billing_status":"charged","charged_amount":0.1}]}}`
	for _, stream := range []bool{false, true} {
		for _, status := range []int{200, 403} {
			for _, chunk := range []int{1, 7, 4096} {
				t.Run(strings.Join([]string{http.StatusText(status), map[bool]string{true: "sse", false: "json"}[stream], string(rune('a' + chunk%26))}, "/"), func(t *testing.T) {
					s, c, repo, account := rejectionTestGateway(t)
					request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
					observe, err := s.prepareGrokRejection(request, account, c)
					require.NoError(t, err)
					require.NotNil(t, observe)
					body, ct := receipt, "application/json"
					if stream {
						ct = "text/event-stream"
						body = "event: ping\r\ndata: " + receipt + "\r\n\r\nevent: response.failed\ndata: " + receipt + "\n\ndata: [DONE]\n\n"
					}
					response := &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{ct}}, Body: &rejectionChunkReader{data: []byte(body), size: chunk}, Request: request}
					require.NoError(t, observe(response))
					actual, err := io.ReadAll(response.Body)
					require.NoError(t, err)
					require.NoError(t, response.Body.Close())
					require.Equal(t, body, string(actual))
					require.Equal(t, float64(1), repo.totals)
					require.True(t, GrokNormalBillingReplaced(c))
					require.Equal(t, int64(1), repo.commands[0].Usage.UserID)
					result := &OpenAIForwardResult{}
					stampGrokRejection(c, result)
					require.True(t, result.GrokNormalBillingReplaced)
					// A subsequent normal attempt is billed normally, including after failover.
					_, err = s.prepareGrokRejection(request, account, c)
					require.NoError(t, err)
					require.False(t, GrokNormalBillingReplaced(c))
				})
			}
		}
	}
}

func TestGrokRejectionTransportTrustAndOwnership(t *testing.T) {
	s, c, repo, account := rejectionTestGateway(t)
	request, _ := http.NewRequest(http.MethodPost, "https://untrusted.example/v1/responses", nil)
	observe, err := s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	require.Nil(t, observe)
	request, _ = http.NewRequest(http.MethodGet, "https://supe.example/v1/videos/foreign", nil)
	_, err = s.prepareGrokRejection(request, account, c)
	require.Error(t, err)
	require.Empty(t, repo.commands)
	repo.owns = true
	observe, err = s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	require.NotNil(t, observe)
}

func TestGrokRejectionStreamTruncatedPreservesReceipt(t *testing.T) {
	body := "data: {\"grok_rejection_billing\":{\"charges\":[]}}\n\ndata: {truncated"
	events, incomplete := 0, 0
	reader := &grokRejectionBody{source: io.NopCloser(strings.NewReader(body)), stream: true, process: func([]byte) error { events++; return nil }, onIncomplete: func() error { incomplete++; return nil }}
	actual, err := io.ReadAll(reader)
	require.NoError(t, err)
	_ = reader.Close()
	require.Equal(t, body, string(actual))
	require.Equal(t, 1, events)
	require.Equal(t, 1, incomplete)
}

func TestGrokRejectionGeneratedTextCannotBill(t *testing.T) {
	s, c, repo, account := rejectionTestGateway(t)
	request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
	observe, err := s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	body := `{"output":[{"content":[{"text":"grok_rejection_billing"}],"grok_rejection_billing":{"charges":[{"billing_request_id":"fake","billing_status":"charged","charged_amount":100}]}}]}`
	response := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}
	require.NoError(t, observe(response))
	_, err = io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Empty(t, repo.commands)
}

func TestGrokRejectionPersistenceFailures(t *testing.T) {
	for _, enqueueFails := range []bool{false, true} {
		t.Run(map[bool]string{true: "inbox_failure", false: "settlement_recovery"}[enqueueFails], func(t *testing.T) {
			s, c, repo, account := rejectionTestGateway(t)
			repo.failEnqueue = enqueueFails
			repo.failSettle = !enqueueFails
			request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
			observe, err := s.prepareGrokRejection(request, account, c)
			require.NoError(t, err)
			response := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"normal_billing":"replaced","grok_rejection_billing":{"charges":[{"billing_request_id":"a","billing_status":"charged","charged_amount":0.1}]}}`))}
			require.NoError(t, observe(response))
			_, err = io.ReadAll(response.Body)
			if enqueueFails {
				require.Error(t, err)
				require.Empty(t, repo.commands)
			} else {
				require.NoError(t, err)
				require.Len(t, repo.commands, 1)
			}
			require.True(t, GrokNormalBillingReplaced(c))
			require.Zero(t, repo.totals)
		})
	}
}

func TestGrokRejectionDisabledAndEmptyWallet(t *testing.T) {
	s, c, _, account := rejectionTestGateway(t)
	request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
	value, _ := c.Get("api_key")
	value.(*APIKey).User.Balance = 0
	_, err := s.prepareGrokRejection(request, account, c)
	require.Error(t, err)
	cache := s.channelService.cache.Load().(*channelCache)
	cache.channelByGroupID[3].FeaturesConfig = map[string]any{}
	observe, err := s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	require.Nil(t, observe)
}

type rejectionHTTPFake struct {
	HTTPUpstream
	response *http.Response
	calls    int
}

func (f *rejectionHTTPFake) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	f.calls++
	f.response.Request = req
	return f.response, nil
}

func TestGrokRejectionHTTPTransportInterceptsBeforePingFilter(t *testing.T) {
	s, c, repo, account := rejectionTestGateway(t)
	receipt := `{"grok_rejection_billing":{"charges":[{"billing_request_id":"ping-fee","billing_status":"charged","charged_amount":0.10}]}}`
	body := "event: ping\ndata: " + receipt + "\n\ndata: [DONE]\n\n"
	transport := &rejectionHTTPFake{response: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}}
	s.httpUpstream = transport
	request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
	response, err := s.doOpenAIUpstream(request, "", account, c)
	require.NoError(t, err)
	filtered := newGrokResponsesBillingPingFilterBody(response.Body, account, 1<<20)
	_, err = io.ReadAll(filtered)
	require.NoError(t, err)
	require.NoError(t, filtered.Close())
	require.Equal(t, 1, transport.calls)
	require.Equal(t, float64(1), repo.totals)
	require.Equal(t, "ping-fee", repo.commands[0].Receipt.BillingRequestID)
}

func TestGrokRejectionSSEMultilineAndCarriageReturn(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", "\r"} {
		body := strings.Join([]string{"event: error", `data: {"normal_billing":"replaced",`, `data: "grok_rejection_billing":{"charges":[]}}`, "", "data: [DONE]", "", ""}, ending)
		events := [][]byte{}
		pending := 0
		reader := &grokRejectionBody{source: &rejectionChunkReader{data: []byte(body), size: 1}, stream: true, process: func(data []byte) error { events = append(events, append([]byte(nil), data...)); return nil }, onIncomplete: func() error { pending++; return nil }}
		actual, err := io.ReadAll(reader)
		require.NoError(t, err)
		_ = reader.Close()
		require.Equal(t, body, string(actual))
		require.Len(t, events, 2)
		require.Zero(t, pending)
		_, replaced, err := ParseGrokRejectionReceipts(events[0], GrokRejectionConfig{})
		require.NoError(t, err)
		require.True(t, replaced)
	}
}

func TestGrokRejectionUsesAuthenticatedPinnedVideoGroup(t *testing.T) {
	s, c, _, account := rejectionTestGateway(t)
	value, _ := c.Get("api_key")
	key := value.(*APIKey)
	key.GroupIDs = []int64{3, 7}
	selected := *key
	groupID := int64(7)
	selected.GroupID = &groupID
	c.Set("ops_fallback_api_key", &selected)
	cache := s.channelService.cache.Load().(*channelCache)
	cache.channelByGroupID[7] = cache.channelByGroupID[3]
	delete(cache.channelByGroupID, 3)
	request, _ := http.NewRequest(http.MethodPost, "https://supe.example/v1/responses", nil)
	observe, err := s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	require.NotNil(t, observe)
	selected.UserID++
	observe, err = s.prepareGrokRejection(request, account, c)
	require.NoError(t, err)
	require.Nil(t, observe)
}

type rejectionClosingReader struct {
	started, closed chan struct{}
	once            sync.Once
}

func (r *rejectionClosingReader) Read(p []byte) (int, error) {
	close(r.started)
	<-r.closed
	return copy(p, `{"normal_billing":"replaced"}`), io.EOF
}
func (r *rejectionClosingReader) Close() error { r.once.Do(func() { close(r.closed) }); return nil }
func TestGrokRejectionConcurrentCloseAccountsReceivedBytes(t *testing.T) {
	source := &rejectionClosingReader{started: make(chan struct{}), closed: make(chan struct{})}
	processed := 0
	reader := &grokRejectionBody{source: source, process: func([]byte) error { processed++; return nil }, onIncomplete: func() error { return nil }}
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _ = reader.Read(make([]byte, 1024)) }()
	<-source.started
	require.NoError(t, reader.Close())
	<-readDone
	require.Equal(t, 1, processed)
	require.NoError(t, reader.Close())
}
