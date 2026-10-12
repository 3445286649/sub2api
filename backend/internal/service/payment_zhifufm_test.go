//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func fmReviewTestClient(t *testing.T) *dbent.Client {
	t.Helper()
	dsn := os.Getenv("FM_TEST_POSTGRES_DSN")
	if dsn == "" {
		return newPaymentConfigServiceTestClient(t)
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, "/fm_fixture", u.Path)
	require.Equal(t, "127.0.0.1", u.Hostname())
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	schema := "fm_fixture_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = db.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = db.Close() })
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	scoped, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, scoped)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(context.Background()))
	return client
}

func newFMReviewFixture(t *testing.T) (*PaymentService, *dbent.PaymentOrder, *float64, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	ctx := context.Background()
	client := fmReviewTestClient(t)
	schema, err := os.ReadFile("../../migrations/244_zhifufm_callback_reviews.sql")
	require.NoError(t, err)
	sqlSchema := string(schema)
	if os.Getenv("FM_TEST_POSTGRES_DSN") == "" {
		sqlSchema = strings.ReplaceAll(strings.ReplaceAll(sqlSchema, "NOW()", "CURRENT_TIMESTAMP"), "TIMESTAMPTZ", "TIMESTAMP")
	}
	_, err = client.ExecContext(ctx, sqlSchema)
	require.NoError(t, err)
	calls := new(atomic.Int32)
	mode := new(atomic.Int32)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/queryOutOrder", r.URL.Path)
		if mode.Load() == 1 {
			w.WriteHeader(503)
			return
		}
		if mode.Load() == 2 {
			fmt.Fprint(w, `{"success":false,"code":403}`)
			return
		}
		amount := "80"
		if mode.Load() == 3 {
			amount = "1"
		}
		fmt.Fprintf(w, `{"success":true,"code":200,"data":{"merchantNum":"123","orderNo":%q,"orderId":"platform-trade","amount":%q,"orderState":"4"}}`, r.URL.Query().Get("orderNo"), amount)
	}))
	t.Cleanup(server.Close)
	// Non-parallel fixture; parallel Go tests remain paused until cleanup restores it.
	savedTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = savedTransport })
	cfg := map[string]string{"pid": "123", "pkey": "fixture-secret", "apiBase": server.URL, "queryBase": server.URL, "notifyUrl": "https://site.test/api/v1/payment/webhook/zhifufm", "returnUrl": "https://site.test/payment/result", "verifyAfterCallback": "true"}
	raw, _ := json.Marshal(cfg)
	instance, err := client.PaymentProviderInstance.Create().SetName("FM fixture").SetProviderKey(payment.ProviderZhifuFM).SetConfig(string(raw)).SetSupportedTypes("alipay,wxpay").Save(ctx)
	require.NoError(t, err)
	order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetPaymentType(payment.TypeAlipay).SetProviderKey(payment.ProviderZhifuFM).SetProviderInstanceID(strconv.FormatInt(instance.ID, 10)).SetPayAmount(80).SetExpiresAt(time.Now().Add(time.Hour)).SetProviderSnapshot(map[string]any{"schema_version": 2, "provider_key": payment.ProviderZhifuFM, "provider_instance_id": strconv.FormatInt(instance.ID, 10), "merchant_id": "123", "fm_verify_after_callback": true}).Save(ctx)
	require.NoError(t, err)
	credited := new(float64)
	userRepo := &mockUserRepo{getByIDUser: &User{ID: order.UserID}}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, order.UserID, id)
		*credited += amount
		return nil
	}
	svc := &PaymentService{entClient: client, registry: payment.NewRegistry(), loadBalancer: payment.NewDefaultLoadBalancer(client, nil), configService: NewPaymentConfigService(client, nil, nil), redeemService: NewRedeemService(&paymentFulfillmentRedeemRepo{}, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil)}
	return svc, order, credited, calls, mode
}
func fmFixtureNotification(o *dbent.PaymentOrder) *payment.PaymentNotification {
	return &payment.PaymentNotification{OrderID: o.OutTradeNo, TradeNo: "platform-trade", Amount: o.PayAmount, Status: payment.NotificationStatusSuccess, Metadata: map[string]string{"pid": "123"}}
}
func fmFixtureReady(t *testing.T, s *PaymentService) {
	t.Helper()
	ctx := context.Background()
	_, err := s.entClient.ExecContext(ctx, `UPDATE payment_fm_query_limits SET next_allowed_at=$1`, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	_, err = s.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET available_at=$1,leased_until=$1 WHERE status IN ('pending','processing')`, time.Now().Add(-time.Minute))
	require.NoError(t, err)
}
func fmFixtureState(t *testing.T, s *PaymentService, id int64) string {
	t.Helper()
	rows, err := s.entClient.QueryContext(context.Background(), `SELECT status FROM payment_fm_reviews WHERE order_id=$1`, id)
	require.NoError(t, err)
	defer rows.Close()
	var state string
	if rows.Next() {
		require.NoError(t, rows.Scan(&state))
	}
	return state
}

func TestFMCallbackDurableReviewCreditsOnce(t *testing.T) {
	svc, o, credited, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	n := fmFixtureNotification(o)
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderZhifuFM))
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderZhifuFM))
	require.Zero(t, *credited)
	require.Zero(t, calls.Load())
	require.Equal(t, "pending", fmFixtureState(t, svc, o.ID))
	// A newly constructed service resumes the persisted callback, even if disabled.
	id, _ := strconv.ParseInt(*o.ProviderInstanceID, 10, 64)
	_, err := svc.configService.UpdateProviderInstance(ctx, id, UpdateProviderInstanceRequest{Enabled: ptrBoolFM(false)})
	require.NoError(t, err)
	restarted := *svc
	require.NoError(t, restarted.ProcessFMCallbackReview(ctx))
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, o.Amount, *credited)
	require.NoError(t, restarted.HandlePaymentNotification(ctx, n, payment.ProviderZhifuFM))
	require.NoError(t, restarted.ProcessFMCallbackReview(ctx))
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, o.Amount, *credited)
	require.Equal(t, "done", fmFixtureState(t, svc, o.ID))
}

func TestFMSuccessNotificationUsesDurableQueryProof(t *testing.T) {
	if os.Getenv("FM_TEST_POSTGRES_DSN") == "" {
		t.Skip("Telegram outbox requires isolated PostgreSQL; set FM_TEST_POSTGRES_DSN")
	}
	for _, policy := range []string{"query", "disabled", "recovered-proof", "query-rejected"} {
		t.Run(policy, func(t *testing.T) {
			svc, o, credited, calls, mode := newFMReviewFixture(t)
			ctx := context.Background()
			schema, err := os.ReadFile("../../migrations/242_payment_telegram_outbox.sql")
			require.NoError(t, err)
			sqlSchema := string(schema)
			_, err = svc.entClient.ExecContext(ctx, sqlSchema)
			require.NoError(t, err)
			svc.telegramService, _ = newTelegramTestService(t)
			_, err = svc.telegramService.UpdateSettings(ctx, UpdateTelegramSettingsRequest{Enabled: true, BotToken: telegramTestToken, ChatID: "-1001234567890"})
			require.NoError(t, err)
			if policy == "disabled" {
				snap := cloneProviderSnapshot(o.ProviderSnapshot)
				snap["fm_verify_after_callback"] = false
				o, err = svc.entClient.PaymentOrder.UpdateOneID(o.ID).SetProviderSnapshot(snap).Save(ctx)
				require.NoError(t, err)
			}
			require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
			if policy != "disabled" {
				pending, err := svc.GetOrder(ctx, o.ID, o.UserID)
				require.NoError(t, err)
				require.Equal(t, "pending", pending.ProviderSnapshot["fm_review_status"])
				require.Zero(t, calls.Load())
				require.Zero(t, *credited)
			}
			if policy == "recovered-proof" {
				_, err = svc.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET verified=TRUE,attempts=3,deadline=$1`, time.Now().Add(-time.Hour))
				require.NoError(t, err)
			}
			if policy == "query-rejected" {
				mode.Store(3)
			}
			require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			if policy == "query-rejected" {
				require.Zero(t, *credited)
				rows, err := svc.entClient.QueryContext(ctx, `SELECT payload FROM payment_telegram_outbox WHERE order_id=$1`, o.ID)
				require.NoError(t, err)
				defer rows.Close()
				for rows.Next() {
					var raw string
					require.NoError(t, rows.Scan(&raw))
					require.NotContains(t, raw, TelegramOutcomeRechargeSuccess)
				}
				require.NoError(t, rows.Err())
				return
			}
			require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
			rows, err := svc.entClient.QueryContext(ctx, `SELECT payload FROM payment_telegram_outbox WHERE order_id=$1`, o.ID)
			require.NoError(t, err)
			defer rows.Close()
			require.True(t, rows.Next())
			var raw string
			require.NoError(t, rows.Scan(&raw))
			var event TelegramPaymentEvent
			require.NoError(t, json.Unmarshal([]byte(raw), &event))
			require.Equal(t, TelegramOutcomeRechargeSuccess, event.Outcome)
			if policy == "disabled" {
				require.Contains(t, formatTelegramPaymentEvent(event), "FM 未查单（自动查单已关闭）")
			} else {
				require.Contains(t, formatTelegramPaymentEvent(event), "FM 已查单")
			}
			require.False(t, rows.Next(), "repeated callbacks must not queue success twice")
			require.NoError(t, rows.Err())
			require.Equal(t, o.Amount, *credited)
			expectedCalls := int32(0)
			if policy == "query" {
				expectedCalls = 1
			}
			require.Equal(t, expectedCalls, calls.Load())
			completed, err := svc.GetOrder(ctx, o.ID, o.UserID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, completed.Status)
			require.NotContains(t, completed.ProviderSnapshot, "fm_review_status")
		})
	}
}

func TestFMUserStatusWaitsForFulfillmentAndProofCannotBeForged(t *testing.T) {
	svc, o, _, _, _ := newFMReviewFixture(t)
	ctx := context.Background()
	require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
	_, err := svc.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET verified=TRUE,trade_no='other-trade' WHERE order_id=$1`, o.ID)
	require.NoError(t, err)
	reason, err := fmPaymentCompletionReason(ctx, svc.entClient, o)
	require.NoError(t, err)
	require.NotContains(t, reason, "FM 已查单")
	for _, status := range []string{OrderStatusPaid, OrderStatusRecharging} {
		_, err := svc.entClient.PaymentOrder.UpdateOneID(o.ID).SetStatus(status).SetPaidAt(time.Now()).Save(ctx)
		require.NoError(t, err)
		current, err := svc.GetOrder(ctx, o.ID, o.UserID)
		require.NoError(t, err)
		require.Equal(t, "crediting", current.ProviderSnapshot["fm_review_status"])
	}
}
func ptrBoolFM(v bool) *bool { return &v }

func TestFMDoesNotQueryWithoutCallback(t *testing.T) {
	svc, o, _, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	_, err := svc.VerifyOrderByOutTradeNo(ctx, o.OutTradeNo, o.UserID)
	require.NoError(t, err)
	_, err = svc.ReconcilePendingPaymentOrders(ctx)
	require.NoError(t, err)
	_, err = svc.CancelOrder(ctx, o.ID, o.UserID)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessFMCallbackReview(ctx))
	require.Zero(t, calls.Load())
}

func TestFMReviewFailureNeverCreditsAndHasBoundedRetries(t *testing.T) {
	for _, modeValue := range []int32{1, 2, 3} {
		t.Run(fmt.Sprint(modeValue), func(t *testing.T) {
			svc, o, credited, calls, mode := newFMReviewFixture(t)
			ctx := context.Background()
			mode.Store(modeValue)
			require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
			for i := 0; i < 5; i++ {
				fmFixtureReady(t, svc)
				require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			}
			require.Zero(t, *credited)
			require.Equal(t, "review", fmFixtureState(t, svc, o.ID))
			expected := int32(1)
			if modeValue == 1 {
				expected = 3
			}
			require.Equal(t, expected, calls.Load())
			require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
			require.NoError(t, svc.ProcessFMCallbackReview(ctx))
			require.Equal(t, expected, calls.Load())
		})
	}
}

func TestFMExpiredReviewAndConflictingCallbacks(t *testing.T) {
	svc, o, credited, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	n := fmFixtureNotification(o)
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderZhifuFM))
	n.TradeNo = "conflict"
	require.Error(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderZhifuFM))
	_, err := svc.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET deadline=$1`, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, svc.ProcessFMCallbackReview(ctx))
	require.Zero(t, calls.Load())
	require.Zero(t, *credited)
}

func TestFMVerifiedProofRecoversWithoutRequery(t *testing.T) {
	svc, o, credited, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
	_, err := svc.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET verified=TRUE,attempts=3,deadline=$1`, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	_, err = svc.entClient.PaymentOrder.UpdateOneID(o.ID).SetStatus(OrderStatusExpired).SetUpdatedAt(time.Now().Add(-time.Hour)).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.ProcessFMCallbackReview(ctx))
	require.Zero(t, calls.Load())
	require.Equal(t, o.Amount, *credited)
}

func TestFMSourceIsExplicitAndCannotFallBack(t *testing.T) {
	svc, o, _, _, _ := newFMReviewFixture(t)
	ctx := context.Background()
	repo := &paymentConfigSettingRepoStub{values: map[string]string{SettingPaymentVisibleMethodAlipaySource: VisibleMethodSourceFMAlipay}}
	svc.configService.settingRepo = repo
	fm, err := svc.entClient.PaymentProviderInstance.Get(ctx, 1)
	require.NoError(t, err)
	_, err = svc.entClient.PaymentProviderInstance.Create().SetName("EasyPay").SetProviderKey(payment.TypeEasyPay).SetSupportedTypes("alipay").SetConfig(`{}`).Save(ctx)
	require.NoError(t, err)
	sources, err := svc.configService.GetAvailableMethodLimits(ctx)
	require.NoError(t, err)
	require.Contains(t, sources.Methods, "alipay")
	require.NotContains(t, sources.Methods, "zhifufm")
	instance, err := svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.NoError(t, err)
	require.Equal(t, fm.ID, instance.ID)
	repo.values[SettingPaymentVisibleMethodAlipayEnabled] = "false"
	_, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.Error(t, err)
	repo.values[SettingPaymentVisibleMethodAlipayEnabled] = "true"
	id, _ := strconv.ParseInt(*o.ProviderInstanceID, 10, 64)
	_, err = svc.configService.UpdateProviderInstance(ctx, id, UpdateProviderInstanceRequest{Enabled: ptrBoolFM(false)})
	require.NoError(t, err)
	_, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.Error(t, err)
	masked, err := svc.configService.ListProviderInstancesWithConfig(ctx)
	require.NoError(t, err)
	require.NotContains(t, masked[0].Config, "pkey")
	_, err = svc.configService.UpdateProviderInstance(ctx, id, UpdateProviderInstanceRequest{Enabled: ptrBoolFM(true)})
	require.NoError(t, err)
	repo.values[SettingPaymentVisibleMethodAlipaySource] = ""
	instance, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.NoError(t, err)
	require.Equal(t, payment.TypeEasyPay, instance.ProviderKey, "adding FM must preserve the existing provider until explicitly selected")
	require.False(t, fmAmountsEqual(80, 80.01))
	require.False(t, fmAmountsEqual(80.001, 80.001))
}

func TestFMCallbackOnlyPolicyStillPreventsTradeReuse(t *testing.T) {
	svc, o, credited, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	snap := cloneProviderSnapshot(o.ProviderSnapshot)
	snap["fm_verify_after_callback"] = false
	o, err := svc.entClient.PaymentOrder.UpdateOneID(o.ID).SetProviderSnapshot(snap).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
	require.Equal(t, o.Amount, *credited)
	require.Zero(t, calls.Load())
	another := createPendingEasyPayFulfillmentOrder(t, ctx, svc.entClient)
	another, err = svc.entClient.PaymentOrder.UpdateOneID(another.ID).SetProviderKey(payment.ProviderZhifuFM).SetProviderInstanceID(*o.ProviderInstanceID).SetProviderSnapshot(snap).Save(ctx)
	require.NoError(t, err)
	require.Error(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(another), payment.ProviderZhifuFM))
	require.Equal(t, o.Amount, *credited)
}

func TestFMMerchantCooldownSharedByWorkers(t *testing.T) {
	svc, o, _, _, _ := newFMReviewFixture(t)
	ctx := context.Background()
	require.NoError(t, svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM))
	task, err := svc.claimFMReview(ctx)
	require.NoError(t, err)
	require.NotNil(t, task)
	_, err = svc.entClient.ExecContext(ctx, `UPDATE payment_fm_reviews SET status='pending',available_at=$1 WHERE order_id=$2`, time.Now().Add(-time.Minute), o.ID)
	require.NoError(t, err)
	other := *svc
	task, err = other.claimFMReview(ctx)
	require.NoError(t, err)
	require.Nil(t, task)
}

func TestFMPostgresConcurrentCallbacksAndClaims(t *testing.T) {
	if os.Getenv("FM_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires isolated local PostgreSQL fixture")
	}
	svc, o, credited, calls, _ := newFMReviewFixture(t)
	ctx := context.Background()
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			errors <- svc.HandlePaymentNotification(ctx, fmFixtureNotification(o), payment.ProviderZhifuFM)
		}()
	}
	for i := 0; i < 8; i++ {
		require.NoError(t, <-errors)
	}
	tasks := make(chan *fmReviewTask, 8)
	for i := 0; i < 8; i++ {
		go func() { task, err := svc.claimFMReview(ctx); errors <- err; tasks <- task }()
	}
	claimed := 0
	for i := 0; i < 8; i++ {
		require.NoError(t, <-errors)
		if <-tasks != nil {
			claimed++
		}
	}
	require.Equal(t, 1, claimed)
	require.Zero(t, *credited)
	require.Zero(t, calls.Load())
}
