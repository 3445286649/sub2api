package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
)

const (
	// Orders whose quote window closed within this duration are reconciled on every cycle.
	bepusdtReconcileRecentWindow = 6 * time.Hour
	bepusdtReconcileRecentLimit  = 20
	// Older unpaid orders are swept slowly. A late payment stays recoverable
	// because the gateway keeps its evidence and its callback outbox retries.
	bepusdtReconcileStaleLimit = 5

	// Minimum spacing between user-triggered gateway queries for one order.
	bepusdtVerifyMinInterval = 5 * time.Second
)

var (
	bepusdtVerifyLast  sync.Map
	bepusdtVerifyCalls atomic.Uint64
)

// bepusdtVerifyAllowed throttles user-triggered verification per gateway order.
// Every verification costs a signed gateway query, and the frontend interval
// alone cannot stop a client that calls the API directly. The throttle is per
// instance, which still bounds the total rate to the instance count.
func bepusdtVerifyAllowed(orderID int64, now time.Time) bool {
	if last, ok := bepusdtVerifyLast.Load(orderID); ok {
		if t, ok := last.(time.Time); ok && now.Sub(t) < bepusdtVerifyMinInterval {
			return false
		}
	}
	bepusdtVerifyLast.Store(orderID, now)
	if bepusdtVerifyCalls.Add(1)%256 == 0 {
		bepusdtVerifyLast.Range(func(key, value any) bool {
			if t, ok := value.(time.Time); ok && now.Sub(t) > 10*bepusdtVerifyMinInterval {
				bepusdtVerifyLast.Delete(key)
			}
			return true
		})
	}
	return true
}

// bepusdtGatewayUnavailable reports whether err means the gateway could not
// answer at all (transport failure, timeout, rejected credentials, overload),
// as opposed to a definite answer about one order.
func bepusdtGatewayUnavailable(err error) bool {
	var upstream *provider.BepusdtAPIError
	if errors.As(err, &upstream) {
		return upstream.Status >= 500 || upstream.Status == 401 || upstream.Status == 403 || upstream.Status == 429
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// ReconcileBepusdtOrders re-queries unpaid gateway orders. Recent orders are
// checked first on every cycle; older ones are swept with a cursor. When the
// gateway is unavailable the cycle stops without advancing the cursor, so a
// gateway outage neither burns the time budget nor skips orders.
func (s *PaymentService) ReconcileBepusdtOrders(ctx context.Context) (int, error) {
	s.cryptoReconcileMu.Lock()
	defer s.cryptoReconcileMu.Unlock()
	cutoff := time.Now().Add(-bepusdtReconcileRecentWindow)
	unpaid := func() *dbent.PaymentOrderQuery {
		return s.entClient.PaymentOrder.Query().Where(paymentorder.ProviderKeyEQ(payment.ProviderBepusdt), paymentorder.PaidAtIsNil(), paymentorder.StatusIn(OrderStatusPending, OrderStatusExpired, OrderStatusCancelled))
	}
	recent, err := unpaid().Where(paymentorder.ExpiresAtGT(cutoff)).Order(dbent.Desc(paymentorder.FieldID)).Limit(bepusdtReconcileRecentLimit).All(ctx)
	if err != nil {
		return 0, err
	}
	stale, err := unpaid().Where(paymentorder.IDGT(s.bepusdtReconcileCursor), paymentorder.ExpiresAtLTE(cutoff)).Order(dbent.Asc(paymentorder.FieldID)).Limit(bepusdtReconcileStaleLimit).All(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	run := func(orders []*dbent.PaymentOrder, sweep bool) error {
		for _, order := range orders {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := s.reconcileBepusdtOrder(ctx, order); err != nil {
				if bepusdtGatewayUnavailable(err) {
					return fmt.Errorf("gateway unavailable, reconcile paused until next cycle: %w", err)
				}
				slog.Warn("gateway reconcile failed", "order_id", order.ID, "error", err)
			} else {
				count++
			}
			if sweep {
				s.bepusdtReconcileCursor = order.ID
			}
		}
		return nil
	}
	if err := run(recent, false); err != nil {
		return count, err
	}
	if err := run(stale, true); err != nil {
		return count, err
	}
	if len(stale) < bepusdtReconcileStaleLimit {
		s.bepusdtReconcileCursor = 0
	}
	return count, nil
}
