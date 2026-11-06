//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBepusdtVerifyAllowedThrottlesPerOrder(t *testing.T) {
	id := time.Now().UnixNano()
	now := time.Now()
	require.True(t, bepusdtVerifyAllowed(id, now))
	require.False(t, bepusdtVerifyAllowed(id, now.Add(2*time.Second)))
	require.True(t, bepusdtVerifyAllowed(id+1, now.Add(2*time.Second)))
	require.True(t, bepusdtVerifyAllowed(id, now.Add(6*time.Second)))
}

func TestBepusdtGatewayUnavailableClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"server error", &provider.BepusdtAPIError{Status: 503}, true},
		{"bad gateway wrapped", fmt.Errorf("query: %w", &provider.BepusdtAPIError{Status: 502}), true},
		{"rejected credentials", &provider.BepusdtAPIError{Status: 401}, true},
		{"throttled", &provider.BepusdtAPIError{Status: 429}, true},
		{"unknown order", &provider.BepusdtAPIError{Status: 404}, false},
		{"definitive rejection", &provider.BepusdtAPIError{Status: 400}, false},
		{"transport timeout", &net.DNSError{IsTimeout: true}, true},
		{"deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), true},
		{"quote validation", errors.New("bepusdt order does not match local immutable intent"), false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, bepusdtGatewayUnavailable(tc.err), tc.name)
	}
}

func TestCheckGatewayCreateRateCapsUnpaidOrders(t *testing.T) {
	ctx := context.Background()
	svc, base, _, _ := bepusdtTestSetup(t, ctx, "http://127.0.0.1:1")
	tx, err := svc.entClient.Tx(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.checkGatewayCreateRate(ctx, tx, base.UserID))
	require.NoError(t, tx.Rollback())
	for i := 0; i < gatewayUnpaidCreateLimit-1; i++ {
		_, err := svc.entClient.PaymentOrder.Create().SetUserID(base.UserID).SetUserEmail(base.UserEmail).SetUserName(base.UserName).SetAmount(7).SetPayAmount(7).SetRechargeCode("PAY-" + uuid.NewString()).SetOutTradeNo("sub2_rate_" + uuid.NewString()).SetPaymentType(payment.TypeUSDTGateway).SetProviderKey(payment.ProviderBepusdt).SetProviderInstanceID(psStringValue(base.ProviderInstanceID)).SetClientRequestID(uuid.NewString()).SetProviderSnapshot(base.ProviderSnapshot).SetPaymentTradeNo("").SetStatus(OrderStatusCancelled).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("example.com").Save(ctx)
		require.NoError(t, err)
	}
	tx, err = svc.entClient.Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	err = svc.checkGatewayCreateRate(ctx, tx, base.UserID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GATEWAY_CREATE_RATE_LIMITED")
}
