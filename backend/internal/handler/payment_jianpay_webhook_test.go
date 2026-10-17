//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestJianPayWebhookEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.Equal(t, "local-order", extractOutTradeNo(`{"merchantOrderNo":"local-order","orderId":"upstream"}`, payment.ProviderJianPay))
	require.Empty(t, extractOutTradeNo(`{"merchantOrderNo":123}`, payment.ProviderJianPay))
	require.Empty(t, extractOutTradeNo(`{"orderId":"upstream"}`, payment.ProviderJianPay))
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"orderId":"upstream"}`, http.StatusBadRequest}, {strings.Repeat("x", maxWebhookBodySize+1), http.StatusRequestEntityTooLarge}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payment/webhook/jianpay", strings.NewReader(tc.body))
		(&PaymentWebhookHandler{}).JianPayNotify(c)
		require.Equal(t, tc.status, w.Code)
		require.NotEqual(t, "success", w.Body.String())
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeSuccessResponse(c, payment.ProviderJianPay)
	require.Equal(t, "success", w.Body.String())
}
