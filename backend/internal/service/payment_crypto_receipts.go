package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptopaymentreceipt"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
)

func lockCryptoPaymentOrder(ctx context.Context, client *dbent.Client, id int64) error {
	if client.Driver().Dialect() == dialect.Postgres {
		_, err := client.PaymentOrder.Query().Where(paymentorder.IDEQ(id)).ForUpdate().Only(ctx)
		return err
	}
	_, err := client.ExecContext(ctx, "UPDATE payment_orders SET id=id WHERE id=?", id)
	return err
}

type cryptoReceiptEvidence struct {
	chain, hash, contract, atomic, source string
	index                                 *int64
}

func cryptoEvidence(order *dbent.PaymentOrder, hash, key string, metadata map[string]string) (cryptoReceiptEvidence, error) {
	evidence := cryptoReceiptEvidence{chain: "bsc_mainnet", hash: strings.ToLower(strings.TrimSpace(hash)), source: key}
	if key == payment.ProviderBepusdt {
		var quote provider.BepusdtOrder
		if json.Unmarshal([]byte(metadata["bepusdt_order"]), &quote) != nil || quote.Payment == nil {
			return evidence, errors.New("missing gateway payment evidence")
		}
		evidence.chain, evidence.contract, evidence.atomic = quote.Network, quote.TokenContract, quote.Payment.AmountAtomic
		evidence.index = &quote.Payment.EventIndex
	} else {
		evidence.contract = "0x55d398326f99059ff775485246999027b3197955"
		amount := metadata["token_amount"]
		if amount == "" {
			nested, _ := order.ProviderSnapshot[payment.TypeUSDTBSC].(map[string]any)
			amount = psSnapshotStringValue(nested["token_amount"])
		}
		if amount != "" {
			atomic, err := provider.BepusdtAmountAtomic(amount, 18)
			if err != nil {
				return evidence, err
			}
			evidence.atomic = atomic
		}
	}
	if evidence.chain == "bsc_mainnet" && !cryptoTxHashPattern.MatchString(evidence.hash) {
		return evidence, errors.New("invalid BSC payment transaction hash")
	}
	if evidence.chain == "tron_mainnet" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(evidence.hash) {
		return evidence, errors.New("invalid TRON payment transaction hash")
	}
	return evidence, nil
}

// Receipt uniqueness, PAID and audit are committed before acknowledging a payment.
func (s *PaymentService) acceptCryptoPayment(ctx context.Context, order *dbent.PaymentOrder, hash string, paid float64, key string, metadata map[string]string) error {
	if !isValidProviderAmount(paid) || math.Abs(paid-order.PayAmount) > paymentAmountToleranceForCurrency(PaymentOrderCurrency(order)) {
		return errors.New("crypto fiat payment amount mismatch")
	}
	evidence, err := cryptoEvidence(order, hash, key, metadata)
	if err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	client := tx.Client()
	if err = lockCryptoPaymentOrder(ctx, client, order.ID); err != nil {
		return err
	}
	current, err := tx.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		return err
	}
	if key == payment.ProviderBepusdt {
		var quote provider.BepusdtOrder
		if json.Unmarshal([]byte(metadata["bepusdt_order"]), &quote) != nil {
			return errors.New("invalid gateway receipt")
		}
		if err = s.validateBepusdtQuote(current, &quote); err != nil {
			return err
		}
	}
	existing, err := tx.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.ChainIDEQ(evidence.chain), cryptopaymentreceipt.TxHashEQ(evidence.hash)).Only(ctx)
	if err == nil {
		if existing.OrderID != current.ID {
			return errors.New("chain transaction is already accepted for another order")
		}
		if current.PaidAt == nil {
			return errors.New("payment receipt exists without durable paid state")
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		s.tryCryptoFulfillment(ctx, current.ID)
		return nil
	}
	if !dbent.IsNotFound(err) {
		return err
	}
	// Until the bounded backfill has caught up, paid legacy rows still reserve
	// their transaction hash, including failed fulfillment and refunded orders.
	if evidence.chain == "bsc_mainnet" {
		used, lookupErr := tx.PaymentOrder.Query().Where(paymentorder.IDNEQ(current.ID), paymentorder.PaidAtNotNil(), paymentorder.PaymentTradeNoEqualFold(evidence.hash), paymentorder.Or(paymentorder.PaymentTypeEQ(payment.TypeUSDTBSC), paymentorder.ProviderKeyEQ(payment.TypeUSDTBSC))).Exist(ctx)
		if lookupErr != nil {
			return lookupErr
		}
		if used {
			return errors.New("chain transaction was used by a historical legacy order")
		}
	}
	if current.PaidAt != nil {
		return errors.New("paid order cannot accept a different chain transaction")
	}
	if key == payment.TypeUSDTBSC {
		grace := time.Now().Add(-paymentGraceMinutes * time.Minute)
		if !current.ExpiresAt.After(grace) {
			return errors.New("legacy crypto order is beyond grace period")
		}
		if current.Status != OrderStatusPending && current.Status != OrderStatusCancelled && !(current.Status == OrderStatusExpired && !current.UpdatedAt.Before(grace)) {
			return errors.New("legacy crypto order cannot accept payment in current status")
		}
	}
	if key == payment.ProviderBepusdt {
		var quote provider.BepusdtOrder
		_ = json.Unmarshal([]byte(metadata["bepusdt_order"]), &quote)
		if err = s.validateBepusdtQuote(current, &quote); err != nil {
			return err
		}
		if current.Status != OrderStatusPending && current.Status != OrderStatusExpired && current.Status != OrderStatusCancelled {
			return errors.New("gateway order cannot accept payment in current status")
		}
	}
	if evidence.chain == "bsc_mainnet" {
		legacyPaid, lookupErr := tx.PaymentOrder.Query().Where(paymentorder.IDNEQ(current.ID), paymentorder.PaidAtNotNil(), paymentorder.PaymentTradeNoEqualFold(evidence.hash), paymentorder.Or(paymentorder.PaymentTypeEQ(payment.TypeUSDTBSC), paymentorder.ProviderKeyEQ(payment.TypeUSDTBSC))).Exist(ctx)
		if lookupErr != nil {
			return lookupErr
		}
		if legacyPaid {
			return errors.New("chain transaction is already attached to a legacy BSC payment")
		}
	}
	receipt := tx.CryptoPaymentReceipt.Create().SetChainID(evidence.chain).SetTxHash(evidence.hash).SetOrderID(current.ID).SetTokenContract(evidence.contract).SetAmountAtomic(evidence.atomic).SetSource(evidence.source).SetNillableEventIndex(evidence.index)
	if _, err = receipt.Save(ctx); err != nil {
		_ = tx.Rollback()
		if dbent.IsConstraintError(err) {
			existing, lookupErr := s.entClient.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.ChainIDEQ(evidence.chain), cryptopaymentreceipt.TxHashEQ(evidence.hash)).Only(ctx)
			if lookupErr == nil {
				if existing.OrderID != current.ID {
					return errors.New("chain transaction is already accepted for another order")
				}
				if latest, getErr := s.entClient.PaymentOrder.Get(ctx, current.ID); getErr == nil && latest.PaidAt != nil {
					return s.executeFulfillment(ctx, current.ID)
				}
			}
		}
		return fmt.Errorf("reserve chain payment receipt: %w", err)
	}
	next := mergePaymentProviderMetadata(current.ProviderSnapshot, key, metadata)
	if key == payment.ProviderBepusdt {
		next = cloneProviderSnapshot(current.ProviderSnapshot)
		nested := next[payment.ProviderBepusdt].(map[string]any)
		var quote provider.BepusdtOrder
		_ = json.Unmarshal([]byte(metadata["bepusdt_order"]), &quote)
		nested["order"] = &quote
		nested["create_stage"] = "ready"
	}
	update := tx.PaymentOrder.UpdateOneID(current.ID).SetStatus(OrderStatusPaid).SetPaidAt(time.Now()).SetPaymentTradeNo(evidence.hash).ClearFailedAt().ClearFailedReason()
	if next != nil {
		update.SetProviderSnapshot(next)
	}
	if _, err = update.Save(ctx); err != nil {
		return err
	}
	detail, _ := json.Marshal(map[string]any{"chain_id": evidence.chain, "tx_hash": evidence.hash, "amount_atomic": evidence.atomic})
	if _, err = tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(current.ID, 10)).SetAction("ORDER_PAID").SetOperator(key).SetDetail(string(detail)).Save(ctx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.tryCryptoFulfillment(ctx, current.ID)
	return nil
}
func (s *PaymentService) tryCryptoFulfillment(ctx context.Context, id int64) {
	if err := s.executeFulfillment(ctx, id); err != nil {
		slog.Warn("crypto payment accepted, fulfillment will retry", "order_id", id, "error", err)
	}
}

// Backfill remains idempotent and includes failed fulfillment and refunded orders.
func (s *PaymentService) BackfillLegacyCryptoReceipts(ctx context.Context) (int, error) {
	s.cryptoReconcileMu.Lock()
	defer s.cryptoReconcileMu.Unlock()
	count := 0
	var blocker error
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.IDGT(s.legacyCryptoBackfillCursor), paymentorder.PaidAtNotNil(), paymentorder.Or(paymentorder.PaymentTypeEQ(payment.TypeUSDTBSC), paymentorder.ProviderKeyEQ(payment.TypeUSDTBSC))).Order(dbent.Asc(paymentorder.FieldID)).Limit(200).All(ctx)
	if err != nil {
		return 0, err
	}
	for _, order := range orders {
		s.legacyCryptoBackfillCursor = order.ID
		hash := strings.ToLower(strings.TrimSpace(order.PaymentTradeNo))
		nested, _ := order.ProviderSnapshot[payment.TypeUSDTBSC].(map[string]any)
		if candidate := psSnapshotStringValue(nested["tx_hash"]); candidate != "" {
			hash = strings.ToLower(candidate)
		}
		if !cryptoTxHashPattern.MatchString(hash) {
			blocker = fmt.Errorf("paid legacy crypto order %d lacks a valid transaction hash; manual review is required", order.ID)
			continue
		}
		existing, err := s.entClient.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.ChainIDEQ("bsc_mainnet"), cryptopaymentreceipt.TxHashEQ(hash)).Only(ctx)
		if err == nil {
			if existing.OrderID != order.ID {
				return count, fmt.Errorf("legacy chain transaction conflict between orders %d and %d", existing.OrderID, order.ID)
			}
			continue
		}
		if !dbent.IsNotFound(err) {
			return count, err
		}
		if _, err = s.entClient.CryptoPaymentReceipt.Create().SetChainID("bsc_mainnet").SetTxHash(hash).SetOrderID(order.ID).SetSource("legacy_usdt_bsc").Save(ctx); err != nil {
			return count, err
		}
		count++
	}
	if len(orders) < 200 {
		s.legacyCryptoBackfillCursor = 0
	}
	return count, blocker
}

func (s *PaymentService) RecoverCryptoFulfillment(ctx context.Context) (int, error) {
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.PaidAtNotNil(), paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed, OrderStatusRecharging), paymentorder.UpdatedAtLT(time.Now().Add(-paymentFulfillmentLeaseDuration)), paymentorder.PaymentTypeIn(payment.TypeUSDTBSC, payment.TypeUSDTGateway)).Order(dbent.Asc(paymentorder.FieldUpdatedAt)).Limit(20).All(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, order := range orders {
		proof, err := s.entClient.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.OrderIDEQ(order.ID)).Exist(ctx)
		if err != nil {
			return count, err
		}
		if !proof {
			continue
		}
		if err = s.executeFulfillment(ctx, order.ID); err == nil {
			count++
		} else {
			slog.Warn("crypto fulfillment recovery failed", "order_id", order.ID, "error", err)
		}
	}
	return count, nil
}

func (s *PaymentService) ReconcileBepusdtOrders(ctx context.Context) (int, error) {
	s.cryptoReconcileMu.Lock()
	defer s.cryptoReconcileMu.Unlock()
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.IDGT(s.bepusdtReconcileCursor), paymentorder.ProviderKeyEQ(payment.ProviderBepusdt), paymentorder.PaidAtIsNil(), paymentorder.StatusIn(OrderStatusPending, OrderStatusExpired, OrderStatusCancelled)).Order(dbent.Asc(paymentorder.FieldID)).Limit(20).All(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	if len(orders) == 0 {
		s.bepusdtReconcileCursor = 0
		return 0, nil
	}
	for _, order := range orders {
		if err = ctx.Err(); err != nil {
			return count, err
		}
		s.bepusdtReconcileCursor = order.ID
		if err = s.reconcileBepusdtOrder(ctx, order); err != nil {
			slog.Warn("gateway reconcile failed", "order_id", order.ID, "error", err)
			continue
		}
		count++
	}
	if len(orders) < 20 {
		s.bepusdtReconcileCursor = 0
	}
	return count, nil
}

func cryptoReceiptExplorer(network, hash string) string {
	switch network {
	case "bsc_mainnet":
		return "https://bscscan.com/tx/" + hash
	case "tron_mainnet":
		return "https://tronscan.org/#/transaction/" + hash
	}
	return ""
}
