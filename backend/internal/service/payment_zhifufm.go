package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/google/uuid"
)

const fmQueryTimeout = 10 * time.Second
const fmReviewLease = 30 * time.Second
const fmMaxQueryAttempts = 3

func fmAmountsEqual(a, b float64) bool {
	return isValidProviderAmount(a) && isValidProviderAmount(b) && math.Abs(a-b) < 0.000001 && math.Abs(a*100-math.Round(a*100)) < 0.000001 && math.Abs(b*100-math.Round(b*100)) < 0.000001
}

func (s *PaymentService) fmOrderHasReview(ctx context.Context, id int64) (bool, error) {
	rows, err := s.entClient.QueryContext(ctx, `SELECT status FROM payment_fm_reviews WHERE order_id=$1 AND status IN ('pending','processing','review')`, id)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}

func (s *PaymentService) attachFMReviewStatus(ctx context.Context, o *dbent.PaymentOrder) *dbent.PaymentOrder {
	if !isFMOrder(o) || o.Status == OrderStatusCompleted || psIsRefundStatus(o.Status) {
		return o
	}
	if o.Status == OrderStatusPaid || o.Status == OrderStatusRecharging {
		o.ProviderSnapshot = cloneProviderSnapshot(o.ProviderSnapshot)
		o.ProviderSnapshot["fm_review_status"] = "crediting"
		return o
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT status FROM payment_fm_reviews WHERE order_id=$1`, o.ID)
	if err != nil {
		return o
	}
	defer rows.Close()
	var status string
	if rows.Next() && rows.Scan(&status) == nil && status != "done" {
		o.ProviderSnapshot = cloneProviderSnapshot(o.ProviderSnapshot)
		o.ProviderSnapshot["fm_review_status"] = status
	}
	return o
}

// Read the durable proof, not the current switch or the worker's final status:
// completion notification is queued before the review task becomes "done".
func fmPaymentCompletionReason(ctx context.Context, client *dbent.Client, o *dbent.PaymentOrder) (string, error) {
	if !isFMOrder(o) {
		return "支付校验通过，余额已入账", nil
	}
	rows, err := client.QueryContext(ctx, `SELECT verified,merchant_id,trade_no,amount FROM payment_fm_reviews WHERE order_id=$1`, o.ID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		var verified bool
		var merchant, trade string
		var amount float64
		if err := rows.Scan(&verified, &merchant, &trade, &amount); err != nil {
			return "", err
		}
		if verified && merchant == psSnapshotStringValue(o.ProviderSnapshot["merchant_id"]) && trade == o.PaymentTradeNo && fmAmountsEqual(amount, o.PayAmount) {
			return "FM 已查单，付款核验通过，余额已入账", nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if verify, ok := o.ProviderSnapshot["fm_verify_after_callback"].(bool); ok && !verify {
		return "FM 未查单（自动查单已关闭），支付回调验签通过，余额已入账", nil
	}
	return "FM 无已查单凭证，余额已入账", nil
}

func isFMOrder(o *dbent.PaymentOrder) bool {
	if o == nil {
		return false
	}
	if psStringValue(o.ProviderKey) == payment.ProviderZhifuFM {
		return true
	}
	snap := psOrderProviderSnapshot(o)
	return snap != nil && snap.ProviderKey == payment.ProviderZhifuFM
}

func (s *PaymentConfigService) fmPendingReviews(ctx context.Context, instanceID int64) (int, error) {
	rows, err := s.entClient.QueryContext(ctx, `SELECT COUNT(*) FROM payment_fm_reviews WHERE provider_instance_id=$1 AND status IN ('pending','processing','review')`, instanceID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		err = rows.Scan(&n)
	}
	return n, err
}

func (s *PaymentService) acceptFMCallback(ctx context.Context, o *dbent.PaymentOrder, tradeNo string, amount float64, metadata map[string]string) error {
	prov, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return err
	}
	fm, ok := prov.(*provider.ZhifuFM)
	if !ok {
		return errors.New("FM order provider mismatch")
	}
	verify := fm.VerifyAfterCallback()
	if frozen, ok := o.ProviderSnapshot["fm_verify_after_callback"].(bool); ok {
		verify = frozen
	}
	if o.Status != OrderStatusPending && o.Status != OrderStatusExpired && o.Status != OrderStatusCancelled {
		return errors.New("FM order cannot accept callback in current status")
	}
	now := time.Now()
	deadline := o.ExpiresAt.Add(paymentGraceMinutes * time.Minute)
	if now.After(deadline) {
		s.notifyPaymentReview(ctx, o, "FM_CALLBACK_TOO_OLD", "FM 订单超出自动复核期限，未查询上游，需人工核查")
		return nil
	}
	if !verify {
		return s.toPaid(ctx, o, tradeNo, amount, payment.ProviderZhifuFM, metadata)
	}
	instanceID, err := strconv.ParseInt(psStringValue(o.ProviderInstanceID), 10, 64)
	if err != nil {
		return errors.New("FM order is missing its provider instance")
	}
	if len(tradeNo) > 100 || metadata["pid"] == "" {
		return errors.New("invalid FM callback identifiers")
	}
	// Retain only validated identifiers, never raw callback/signature/secret.
	_, err = s.entClient.ExecContext(ctx, `INSERT INTO payment_fm_reviews(order_id,provider_instance_id,merchant_id,trade_no,amount,status,available_at,deadline) VALUES($1,$2,$3,$4,$5,'pending',$6,$7) ON CONFLICT(order_id) DO NOTHING`, o.ID, instanceID, metadata["pid"], tradeNo, amount, now, deadline)
	if err != nil {
		return errors.New("unable to persist FM callback verification")
	}
	// Protect the first accepted callback from a conflicting signed callback.
	rows, err := s.entClient.QueryContext(ctx, `SELECT trade_no,amount,merchant_id FROM payment_fm_reviews WHERE order_id=$1`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var savedTrade, merchant string
	var savedAmount float64
	if !rows.Next() {
		return errors.New("FM callback task missing")
	}
	if err = rows.Scan(&savedTrade, &savedAmount, &merchant); err != nil {
		return err
	}
	if savedTrade != tradeNo || merchant != metadata["pid"] || !fmAmountsEqual(savedAmount, amount) {
		return errors.New("conflicting FM callback")
	}
	return nil
}

type fmReviewTask struct {
	orderID                int64
	merchant, trade, lease string
	amount                 float64
	verified               bool
	attempts               int
	deadline               time.Time
}

// Claiming both the merchant cooldown and task in a transaction makes the query
// limit apply across workers, containers and instances using the same merchant.
func (s *PaymentService) claimFMReview(ctx context.Context) (*fmReviewTask, error) {
	now := time.Now()
	rows, err := s.entClient.QueryContext(ctx, `SELECT order_id,merchant_id,trade_no,amount,verified,attempts,deadline FROM payment_fm_reviews WHERE (status='pending' AND available_at<=$1) OR (status='processing' AND leased_until<=$1) ORDER BY available_at,order_id LIMIT 20`, now)
	if err != nil {
		return nil, err
	}
	var tasks []fmReviewTask
	for rows.Next() {
		var t fmReviewTask
		if err = rows.Scan(&t.orderID, &t.merchant, &t.trade, &t.amount, &t.verified, &t.attempts, &t.deadline); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return nil, err
		}
		_, err = tx.Client().ExecContext(ctx, `INSERT INTO payment_fm_query_limits(merchant_id,next_allowed_at) VALUES($1,$2) ON CONFLICT(merchant_id) DO NOTHING`, t.merchant, now)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		// The HTTP timeout plus 3 seconds also protects against overlapping slow calls.
		limitRows, err := tx.Client().QueryContext(ctx, `UPDATE payment_fm_query_limits SET next_allowed_at=$1 WHERE merchant_id=$2 AND next_allowed_at<=$3 RETURNING merchant_id`, now.Add(fmQueryTimeout+3*time.Second), t.merchant, now)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		allowed := limitRows.Next()
		limitRows.Close()
		if !allowed {
			tx.Rollback()
			continue
		}
		t.lease = uuid.NewString()
		claimed, err := tx.Client().QueryContext(ctx, `UPDATE payment_fm_reviews SET status='processing',lease_token=$1,leased_until=$2 WHERE order_id=$3 AND ((status='pending' AND available_at<=$4) OR (status='processing' AND leased_until<=$4)) RETURNING order_id`, t.lease, now.Add(fmReviewLease), t.orderID, now)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		ok := claimed.Next()
		claimed.Close()
		if !ok {
			tx.Rollback()
			continue
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return &t, nil
	}
	return nil, nil
}

// ProcessFMCallbackReview only works on tasks persisted by a verified callback.
func (s *PaymentService) ProcessFMCallbackReview(ctx context.Context) error {
	task, err := s.claimFMReview(ctx)
	if err != nil || task == nil {
		return err
	}
	order, err := s.entClient.PaymentOrder.Get(ctx, task.orderID)
	if err != nil {
		return s.finishFMReview(ctx, task, "review", "FM_ORDER_UNAVAILABLE", false)
	}
	if order.PaidAt != nil {
		if !psIsRefundStatus(order.Status) && order.Status != OrderStatusCompleted {
			if err = s.executeFulfillment(ctx, order.ID); err != nil {
				return err
			}
		}
		return s.finishFMReview(ctx, task, "done", "", false)
	}
	if !isFMOrder(order) {
		return s.finishFMReview(ctx, task, "review", "FM_PROVIDER_MISMATCH", false)
	}
	if !task.verified && (time.Now().After(task.deadline) || task.attempts >= fmMaxQueryAttempts) {
		return s.failFMReview(ctx, task, order, "FM_REVIEW_EXPIRED", false)
	}
	var result *payment.QueryOrderResponse
	if !task.verified {
		prov, err := s.getOrderProvider(ctx, order)
		if err != nil {
			return s.failFMReview(ctx, task, order, "FM_PROVIDER_UNAVAILABLE", false)
		}
		// Persist the attempt before network I/O so a crash cannot reset the budget.
		_, err = s.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET attempts=attempts+1 WHERE order_id=$1 AND lease_token=$2`, task.orderID, task.lease)
		if err != nil {
			return err
		}
		task.attempts++
		queryCtx, cancel := context.WithTimeout(ctx, fmQueryTimeout)
		result, err = prov.QueryOrder(queryCtx, order.OutTradeNo)
		cancel()
		if err != nil {
			var qe *provider.FMQueryError
			retry := errors.As(err, &qe) && qe.Retryable
			return s.failFMReview(ctx, task, order, "FM_QUERY_FAILED", retry)
		}
		if result == nil || result.Status != payment.ProviderStatusPaid {
			return s.failFMReview(ctx, task, order, "FM_NOT_PAID", false)
		}
		if result.TradeNo != task.trade || result.Metadata["pid"] != task.merchant || result.Metadata["out_trade_no"] != order.OutTradeNo {
			return s.failFMReview(ctx, task, order, "FM_IDENTITY_MISMATCH", false)
		}
		if !fmAmountsEqual(result.Amount, task.amount) || !fmAmountsEqual(result.Amount, order.PayAmount) {
			return s.failFMReview(ctx, task, order, "FM_AMOUNT_MISMATCH", false)
		}
		proofUpdate, err := s.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET verified=TRUE WHERE order_id=$1 AND lease_token=$2`, task.orderID, task.lease)
		if err != nil {
			return err
		}
		if n, e := proofUpdate.RowsAffected(); e != nil || n != 1 {
			return errors.New("FM proof lease was lost")
		}
	} else {
		result = &payment.QueryOrderResponse{Amount: task.amount, Metadata: map[string]string{"pid": task.merchant, "out_trade_no": order.OutTradeNo}}
	}
	// Reuse the idempotent fulfillment path; no second provider query.
	if err = s.handlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: task.trade, Amount: result.Amount, Status: payment.NotificationStatusSuccess, Metadata: result.Metadata}, payment.ProviderZhifuFM, true); err != nil {
		fresh, loadErr := s.entClient.PaymentOrder.Get(ctx, order.ID)
		if loadErr != nil || fresh.PaidAt == nil {
			return err // Durable proof remains leased for local recovery, without another FM query.
		}
		// Paid proof is persisted. Recovery retries fulfillment without querying FM.
		return err
	}
	return s.finishFMReview(ctx, task, "done", "", false)
}

func (s *PaymentService) failFMReview(ctx context.Context, t *fmReviewTask, o *dbent.PaymentOrder, code string, retry bool) error {
	retry = retry && t.attempts < fmMaxQueryAttempts && time.Now().Add(15*time.Second).Before(t.deadline)
	status := "review"
	if retry {
		status = "pending"
	}
	if err := s.finishFMReview(ctx, t, status, code, retry); err != nil {
		return err
	}
	s.writeAuditLog(ctx, o.ID, code, "system", map[string]any{"attempts": t.attempts, "retry_scheduled": retry})
	s.notifyPaymentReview(ctx, o, code, "FM 付款复核未通过，余额未入账，需复核；不能据此判定用户造假")
	return nil
}
func (s *PaymentService) finishFMReview(ctx context.Context, t *fmReviewTask, status, code string, retry bool) error {
	next := time.Now()
	if retry {
		next = next.Add(15 * time.Second)
	}
	_, err := s.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET status=$1,last_error=$2,available_at=$3,lease_token=NULL,leased_until=NULL WHERE order_id=$4 AND lease_token=$5`, status, code, next, t.orderID, t.lease)
	return err
}

func (s *PaymentOrderExpiryService) runFMReviews() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			if err := s.paymentSvc.ProcessFMCallbackReview(ctx); err != nil {
				slog.Warn("FM callback review worker failed", "error", fmt.Sprintf("%T", err))
			}
			cancel()
		}
	}
}
