package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAISSEDataPayloadRepairsStringControls(t *testing.T) {
	input := []byte("{\"type\":\"response.output_text.delta\",\"delta\":\"line\nnext\tpart\"}")

	got, changed, err := normalizeOpenAISSEDataPayload(input)

	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, json.Valid(got))
	var decoded struct {
		Delta string `json:"delta"`
	}
	require.NoError(t, json.Unmarshal(got, &decoded))
	require.Equal(t, "line\nnext\tpart", decoded.Delta)
}

func TestNormalizeOpenAISSEDataPayloadLeavesValidJSONByteStable(t *testing.T) {
	input := []byte(`{"type":"response.output_text.delta","delta":"line\nnext"}`)

	got, changed, err := normalizeOpenAISSEDataPayload(input)

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, input, got)
}

func TestForEachOpenAISSEFrameRepairsRawControlInsideJSONString(t *testing.T) {
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\tworld\"}\n\n"
	var payload []byte
	forEachOpenAISSEFrame(body, func(_ string, data []byte) {
		payload = append([]byte(nil), data...)
	})

	require.JSONEq(t, `{"type":"response.output_text.delta","delta":"hello\tworld"}`, string(payload))
}

func TestNormalizeOpenAISSEDataPayloadRejectsStructuralCorruption(t *testing.T) {
	input := []byte(`{"type":"response.output_text.delta","delta":"unterminated}`)

	got, changed, err := normalizeOpenAISSEDataPayload(input)

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, input, got)
}

func TestNormalizeOpenAISSEDataPayloadRejectsControlByteWhenJSONCannotBeRecovered(t *testing.T) {
	input := []byte("{\"delta\":\"unterminated\t}")

	got, changed, err := normalizeOpenAISSEDataPayload(input)

	require.Error(t, err)
	require.False(t, changed)
	require.Equal(t, input, got)
}

func TestNormalizeOpenAIResponseCreatedAtConvertsIntegerValuedFloat(t *testing.T) {
	input := []byte(`{"type":"response.completed","response":{"created_at":1790892388.0}}`)

	got, changed := normalizeOpenAIResponseCreatedAt(input)

	require.True(t, changed)
	require.Equal(t, `{"type":"response.completed","response":{"created_at":1790892388}}`, string(got))
}

func TestNormalizeOpenAIResponseCreatedAtLeavesFractionalValue(t *testing.T) {
	input := []byte(`{"type":"response.completed","response":{"created_at":1790892388.5}}`)

	got, changed := normalizeOpenAIResponseCreatedAt(input)

	require.False(t, changed)
	require.Equal(t, input, got)
}

func TestHandleStreamingResponseRepairsRawControlByteBeforeForwarding(t *testing.T) {
	recorder := newOpenAIResponseFlushRecorder()
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\tworld\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"

	result, err := runOpenAIResponseFlushTest(recorder, io.NopCloser(strings.NewReader(body)), config.GatewayConfig{})

	require.NoError(t, err)
	require.NotNil(t, result)
	output, _ := recorder.snapshot()
	assertOpenAISSEFrames(t, output, []string{"response.output_text.delta", "response.completed"})
	require.Contains(t, output, `"delta":"hello\tworld"`)
}

func TestHandleStreamingResponsePassthroughRepairsRawControlByteBeforeForwarding(t *testing.T) {
	_, recorder, _, err := runPassthroughFlushTest(t,
		io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\tworld\"}\n\n"+
			"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")), -1)

	require.NoError(t, err)
	assertOpenAISSEFrames(t, recorder.Body.String(), []string{"response.output_text.delta", "response.completed"})
	require.Contains(t, recorder.Body.String(), `"delta":"hello\tworld"`)
}

func TestHandleStreamingResponseNormalizesFloatCreatedAt(t *testing.T) {
	recorder := newOpenAIResponseFlushRecorder()
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"created_at\":1790892388.0,\"status\":\"in_progress\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"created_at\":1790892388.0,\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"

	result, err := runOpenAIResponseFlushTest(recorder, io.NopCloser(strings.NewReader(body)), config.GatewayConfig{})

	require.NoError(t, err)
	require.NotNil(t, result)
	output, _ := recorder.snapshot()
	require.NotContains(t, output, "1790892388.0")
	require.Contains(t, output, "1790892388")
}

func TestScanCCStreamRepairsRawControlByteBeforeUnmarshal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\tworld\"}}]}\n\n" +
				"data: [DONE]\n\n",
		)),
	}

	var chunks int
	state := (&OpenAIGatewayService{}).scanCCStream(c, resp, "test", "rid", time.Now(), func(*apicompat.ChatCompletionsChunk) {
		chunks++
	})

	require.NoError(t, state.Err)
	require.True(t, state.SawDone)
	require.Equal(t, 1, chunks)
}
