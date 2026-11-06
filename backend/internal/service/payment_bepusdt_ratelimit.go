package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Gateway quote slots are permanent: every (address, amount) tail is consumed
// even when an order expires or is cancelled. Cap how many unpaid gateway
// orders one user can create per window so a single account cannot burn the
// slot pool for a common recharge amount.
const (
	gatewayUnpaidCreateLimit  = 10
	gatewayUnpaidCreateWindow = time.Hour
)

func (s *PaymentService) checkGatewayCreateRate(ctx context.Context, tx *dbent.Tx, userID int64) error {
	count, err := tx.PaymentOrder.Query().Where(
		paymentorder.UserIDEQ(userID),
		paymentorder.PaymentTypeEQ(payment.TypeUSDTGateway),
		paymentorder.PaidAtIsNil(),
		paymentorder.CreatedAtGTE(time.Now().Add(-gatewayUnpaidCreateWindow)),
	).Count(ctx)
	if err != nil {
		return fmt.Errorf("count recent unpaid gateway orders: %w", err)
	}
	if count >= gatewayUnpaidCreateLimit {
		return infraerrors.TooManyRequests("GATEWAY_CREATE_RATE_LIMITED", "too many unpaid crypto orders, try again later").
			WithMetadata(map[string]string{"max": strconv.Itoa(gatewayUnpaidCreateLimit), "window_minutes": strconv.Itoa(int(gatewayUnpaidCreateWindow / time.Minute))})
	}
	return nil
}
