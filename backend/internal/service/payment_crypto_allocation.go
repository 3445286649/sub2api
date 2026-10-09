package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
)

// Allocation and intent persistence share one transaction. A database lock, rather
// than a process mutex, also serializes different instances and provider configs.
func (s *PaymentService) beginCryptoAllocation(ctx context.Context, orderID int64, result *payment.CreatePaymentResponse) (*dbent.Tx, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()
	client := tx.Client()
	if client.Driver().Dialect() == dialect.Postgres {
		_, err = client.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(738294015))
	} else {
		// SQLite tests: obtain the write lock before reading reservations.
		_, err = client.ExecContext(ctx, "UPDATE payment_orders SET id = id WHERE id = ?", orderID)
	}
	if err != nil {
		return nil, fmt.Errorf("lock crypto amount allocation: %w", err)
	}
	orders, err := client.PaymentOrder.Query().Where(
		paymentorder.IDNEQ(orderID),
		paymentorder.Or(paymentorder.PaymentTypeEQ(payment.TypeUSDTBSC), paymentorder.ProviderKeyEQ(payment.TypeUSDTBSC)),
		paymentorder.Or(
			paymentorder.StatusEQ(OrderStatusPending),
			paymentorder.ExpiresAtGTE(time.Now().Add(-2*time.Hour)),
			paymentorder.UpdatedAtGTE(time.Now().Add(-2*time.Hour)),
		),
	).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("read crypto amount reservations: %w", err)
	}
	reserved := make([]string, 0, len(orders))
	for _, order := range orders {
		intent := order.PaymentTradeNo
		if snapshot, ok := order.ProviderSnapshot[payment.TypeUSDTBSC].(map[string]any); ok {
			if value, ok := snapshot["intent_trade_no"].(string); ok && value != "" {
				intent = value
			}
		}
		parts := strings.Split(intent, "|")
		if len(parts) < 5 || !strings.EqualFold(parts[4], result.ReceiveAddress) {
			continue
		}
		amount, valid := provider.USDTBSCIntentTokenAmount(intent)
		if !valid {
			return nil, fmt.Errorf("invalid reserved crypto payment intent for order %d", order.ID)
		}
		reserved = append(reserved, amount)
	}
	// A gateway's exposed quote is permanent even after cancelling or disabling
	// its instance. Switching back to the legacy allocator must not reuse it.
	var cursor int64
	for {
		gatewayOrders, err := client.PaymentOrder.Query().Where(paymentorder.IDGT(cursor), paymentorder.ProviderKeyEQ(payment.ProviderBepusdt)).Order(dbent.Asc(paymentorder.FieldID)).Limit(200).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("read permanent gateway amount reservations: %w", err)
		}
		for _, order := range gatewayOrders {
			cursor = order.ID
			quote := bepusdtSnapshotOrder(order)
			if quote == nil && psSnapshotStringValue(bepusdtSnapshot(order)["network"]) == "bsc_mainnet" && psSnapshotStringValue(bepusdtSnapshot(order)["create_stage"]) != "rejected" {
				return nil, fmt.Errorf("gateway BSC quote for order %d remains unresolved", order.ID)
			}
			if quote == nil || quote.Network != "bsc_mainnet" || !strings.EqualFold(quote.ReceiveAddress, result.ReceiveAddress) {
				continue
			}
			reserved = append(reserved, quote.CryptoAmount)
		}
		if len(gatewayOrders) < 200 {
			break
		}
	}
	if err := provider.ReserveUSDTBSCAmount(result, reserved); err != nil {
		return nil, err
	}
	ok = true
	return tx, nil
}
