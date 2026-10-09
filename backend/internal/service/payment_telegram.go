package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func telegramPaymentEventForOrder(o *dbent.PaymentOrder, outcome string, occurredAt time.Time, reason string) TelegramPaymentEvent {
	event := TelegramPaymentEvent{
		EventKey: fmt.Sprintf("order:%d:%s", o.ID, outcome),
		OrderID:  o.ID, UserID: o.UserID, Username: o.UserName, Email: o.UserEmail,
		OccurredAt: occurredAt, PayAmount: o.PayAmount, Currency: PaymentOrderCurrency(o),
		PaymentType: o.PaymentType, OutTradeNo: o.OutTradeNo, Outcome: outcome, Reason: reason,
		Status: o.Status,
	}
	if outcome == TelegramOutcomeRechargeSuccess {
		event.CreditedAmount, event.BonusAmount = o.Amount, o.BonusAmount
		event.Status = OrderStatusCompleted
		if receipt := PaymentOrderCryptoReceiptOf(o); receipt != nil {
			event.ReceiptAmount, event.CryptoCurrency = receipt.Amount, receipt.Currency
		}
	}
	return event
}

// Alerts describe the affected order; they never attribute an external callback
// to its owner or change that user's account status.
func (s *PaymentService) notifyPaymentReview(ctx context.Context, o *dbent.PaymentOrder, code, reason string) {
	if s.telegramService == nil || o == nil || o.PaidAt != nil ||
		(o.Status != OrderStatusPending && o.Status != OrderStatusCancelled && o.Status != OrderStatusExpired) {
		return
	}
	event := telegramPaymentEventForOrder(o, TelegramOutcomePaymentReview, time.Now(), reason)
	event.EventKey = fmt.Sprintf("order:%d:review:%s", o.ID, code)
	if err := s.telegramService.Enqueue(ctx, event); err != nil {
		slog.Warn("unable to enqueue payment review notification", "order_id", o.ID, "reason", code)
	}
}

func (s *PaymentService) RecordRejectedPaymentCallback(ctx context.Context, providerKey, outTradeNo string) {
	if s.telegramService == nil || len(outTradeNo) == 0 || len(outTradeNo) > 64 {
		return
	}
	order, err := s.entClient.PaymentOrder.Query().Where(
		paymentorder.OutTradeNoEQ(outTradeNo),
		paymentorder.PaidAtIsNil(),
		paymentorder.StatusIn(OrderStatusPending, OrderStatusCancelled, OrderStatusExpired),
		paymentorder.ExpiresAtGT(time.Now().Add(-paymentGraceMinutes*time.Minute)),
	).Only(ctx)
	if err != nil {
		return
	}
	if expectedNotificationProviderKeyForOrder(s.registry, order, "") != providerKey {
		return
	}
	// The event key bounds repeated invalid callbacks to one alert per order.
	// The referenced order identifies the affected user, not the attacker.
	s.notifyPaymentReview(ctx, order, "PAYMENT_CALLBACK_REJECTED", "支付回调未通过验签或参数校验，未入账；不能据此判定订单用户造假")
}

func (s *PaymentService) verifyNewEasyPayNotification(ctx context.Context, o *dbent.PaymentOrder, providerKey, tradeNo string, paid float64) error {
	if s.telegramService == nil || providerKey != payment.TypeEasyPay || o.PaidAt != nil ||
		(o.Status != OrderStatusPending && o.Status != OrderStatusCancelled && o.Status != OrderStatusExpired) {
		return nil
	}
	settings, err := s.telegramService.GetSettings(ctx)
	if err != nil {
		return infraerrors.ServiceUnavailable("PAYMENT_VERIFICATION_UNAVAILABLE", "Payment verification settings are unavailable")
	}
	if !settings.VerifyEasyPayOrders {
		return nil
	}
	provider, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_QUERY_FAILED", "无法读取订单绑定的支付网关，暂停入账，待复核")
	}
	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := provider.QueryOrder(queryCtx, paymentOrderQueryReference(o, provider))
	if err != nil || result == nil {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_QUERY_FAILED", "上游查单失败，暂停入账，待复核；尚不能判定造假")
	}
	if result.Status != payment.ProviderStatusPaid {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_NOT_PAID", "收到成功回调，但上游查单未确认已支付，已拦截入账")
	}
	tolerance := paymentAmountToleranceForCurrency(PaymentOrderCurrency(o))
	if !isValidProviderAmount(result.Amount) || math.Abs(result.Amount-paid) > tolerance || math.Abs(result.Amount-o.PayAmount) > tolerance {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_AMOUNT_MISMATCH", "上游查单金额与支付回调不一致，已拦截入账")
	}
	if strings.TrimSpace(result.TradeNo) == "" || strings.TrimSpace(result.TradeNo) != strings.TrimSpace(tradeNo) {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_TRADE_MISMATCH", "上游查单交易号与支付回调不一致，已拦截入账")
	}
	if err := validateProviderNotificationMetadata(o, providerKey, result.Metadata); err != nil {
		return s.rejectUpstreamPaymentReview(ctx, o, "PAYMENT_UPSTREAM_MERCHANT_MISMATCH", "上游查单商户信息与订单不一致，已拦截入账")
	}
	return nil
}

func (s *PaymentService) rejectUpstreamPaymentReview(ctx context.Context, o *dbent.PaymentOrder, code, reason string) error {
	s.writeAuditLog(ctx, o.ID, code, "system", map[string]any{"reason": reason})
	s.notifyPaymentReview(ctx, o, code, reason)
	return infraerrors.ServiceUnavailable("PAYMENT_REVIEW_REQUIRED", "Payment requires verification; balance has not been credited")
}
