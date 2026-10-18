//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func newJianPayFixture(t *testing.T) (*PaymentService, *dbent.PaymentOrder, *dbent.PaymentProviderInstance, *float64, *atomic.Int32) {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	calls := new(atomic.Int32)
	merchantOrderNo := ""
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/open/payment/pay/info", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if body["orderId"] != "jian-trade" || body["merchantOrderNo"] != nil {
			fmt.Fprint(w, `{"code":1001,"message":"signature verification failed"}`)
			return
		}
		fmt.Fprintf(w, `{"code":1000,"data":{"clientNo":"JP-test","merchantOrderNo":%q,"orderId":"jian-trade","amount":8000,"status":2,"payMethod":"alipay"}}`, merchantOrderNo)
	}))
	t.Cleanup(server.Close)
	savedTransport := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = savedTransport })
	cfg := map[string]string{"clientNo": "JP-test", "merchantKey": "fixture-key", "apiBase": server.URL, "notifyUrl": "https://site.test/api/v1/payment/webhook/jianpay", "returnUrl": "https://site.test/payment/result"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.ProviderJianPay).SetName("简付测试").SetConfig(string(raw)).SetSupportedTypes("alipay").Save(ctx)
	require.NoError(t, err)
	order := createPendingEasyPayFulfillmentOrder(t, ctx, client)
	merchantOrderNo = order.OutTradeNo
	instanceID := strconv.FormatInt(inst.ID, 10)
	order, err = client.PaymentOrder.UpdateOneID(order.ID).SetProviderKey(payment.ProviderJianPay).SetProviderInstanceID(instanceID).SetPaymentTradeNo("jian-trade").SetPaymentType(payment.TypeAlipay).SetPayAmount(80).SetExpiresAt(time.Now().Add(time.Hour)).SetProviderSnapshot(buildPaymentOrderProviderSnapshot(&payment.InstanceSelection{ProviderKey: payment.ProviderJianPay, InstanceID: instanceID, Config: cfg}, CreateOrderRequest{PaymentType: payment.TypeAlipay})).Save(ctx)
	require.NoError(t, err)
	credited := new(float64)
	userRepo := &mockUserRepo{getByIDUser: &User{ID: order.UserID}}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, order.UserID, id)
		*credited += amount
		return nil
	}
	svc := &PaymentService{entClient: client, registry: payment.NewRegistry(), loadBalancer: payment.NewDefaultLoadBalancer(client, nil), configService: NewPaymentConfigService(client, nil, nil), redeemService: NewRedeemService(&paymentFulfillmentRedeemRepo{}, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil)}
	return svc, order, inst, credited, calls
}

func jianPayFixtureNotification(order *dbent.PaymentOrder) *payment.PaymentNotification {
	return &payment.PaymentNotification{OrderID: order.OutTradeNo, TradeNo: "jian-trade", Amount: 80, Status: payment.NotificationStatusSuccess, Metadata: map[string]string{"clientNo": "JP-test", "merchantOrderNo": order.OutTradeNo, "payMethod": "alipay", "amount_cents": "8000"}}
}

func TestJianPayMerchantMethodsDoNotOverlapAcrossMerchants(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewPaymentConfigService(client, nil, []byte("0123456789abcdef0123456789abcdef"))
	config := func(clientNo string) map[string]string {
		return map[string]string{
			"clientNo": clientNo, "merchantKey": "fixture-key",
			"notifyUrl": "https://site.test/api/v1/payment/webhook/jianpay",
			"returnUrl": "https://site.test/payment/result",
		}
	}
	create := func(name, clientNo string, methods ...string) (*dbent.PaymentProviderInstance, error) {
		return svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{
			ProviderKey: payment.ProviderJianPay, Name: name, Config: config(clientNo),
			SupportedTypes: methods, Enabled: true,
		})
	}
	alipay, err := create("Alipay", "JP-alipay", payment.TypeAlipay)
	require.NoError(t, err)
	wechat, err := create("WeChat", "JP-wechat", payment.TypeWxpay)
	require.NoError(t, err)
	svc.settingRepo = &paymentConfigSettingRepoStub{values: map[string]string{
		SettingPaymentVisibleMethodAlipaySource: VisibleMethodSourceJianPayAlipay,
		SettingPaymentVisibleMethodWxpaySource:  VisibleMethodSourceJianPayWechat,
	}}
	lb := newVisibleMethodLoadBalancer(payment.NewDefaultLoadBalancer(client, svc.encryptionKey), svc)
	for _, tc := range []struct {
		method     string
		instanceID int64
		clientNo   string
	}{
		{payment.TypeAlipay, alipay.ID, "JP-alipay"},
		{payment.TypeWxpay, wechat.ID, "JP-wechat"},
	} {
		selected, err := lb.SelectInstance(ctx, "", tc.method, payment.StrategyRoundRobin, 1)
		require.NoError(t, err)
		require.Equal(t, strconv.FormatInt(tc.instanceID, 10), selected.InstanceID)
		require.Equal(t, tc.clientNo, selected.Config["clientNo"])
	}
	_, err = create("Wrong merchant", "JP-other", payment.TypeWxpay)
	require.ErrorContains(t, err, "JianPay payment method")
	_, err = create("Same merchant", "JP-wechat", payment.TypeWxpay)
	require.NoError(t, err)
	_, err = svc.UpdateProviderInstance(ctx, wechat.ID, UpdateProviderInstanceRequest{SupportedTypes: []string{payment.TypeAlipay, payment.TypeWxpay}})
	require.ErrorContains(t, err, "JianPay payment method")
	_, err = svc.UpdateProviderInstance(ctx, alipay.ID, UpdateProviderInstanceRequest{SupportedTypes: []string{payment.TypeAlipay, payment.TypeWxpay}})
	require.ErrorContains(t, err, "JianPay payment method")
}

func TestJianPayPaymentModePersistence(t *testing.T) {
	ctx := context.Background()
	svc := NewPaymentConfigService(newPaymentConfigServiceTestClient(t), nil, []byte(strings.Repeat("k", 32)))
	_, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{ProviderKey: payment.ProviderJianPay, Name: "Fixture", PaymentMode: "redirect"})
	require.Error(t, err)
	for _, initial := range []string{"", "qrcode", "popup"} {
		inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{ProviderKey: payment.ProviderJianPay, Name: "Fixture", SupportedTypes: []string{payment.TypeAlipay}, PaymentMode: initial})
		require.NoError(t, err)
		require.Equal(t, initial, inst.PaymentMode)
		for _, mode := range []string{"popup", "qrcode", "invalid"} {
			updated, err := svc.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{PaymentMode: &mode})
			if mode == "invalid" {
				require.Error(t, err)
				saved, err := svc.entClient.PaymentProviderInstance.Get(ctx, inst.ID)
				require.NoError(t, err)
				require.Equal(t, "qrcode", saved.PaymentMode)
			} else {
				require.NoError(t, err)
				require.Equal(t, mode, updated.PaymentMode)
			}
		}
	}
}

func TestJianPayQueryAndCallbackFulfillOnce(t *testing.T) {
	svc, order, inst, credited, _ := newJianPayFixture(t)
	ctx := context.Background()
	// Disabling a provider stops new orders but must not strand old payments.
	_, err := svc.configService.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	_, err = svc.VerifyOrderByOutTradeNo(ctx, order.OutTradeNo, order.UserID+1)
	require.Error(t, err)
	require.Zero(t, *credited)
	verified, err := svc.VerifyOrderByOutTradeNo(ctx, order.OutTradeNo, order.UserID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, verified.Status)
	require.Equal(t, order.Amount, *credited)
	providers, err := svc.GetWebhookProviders(ctx, payment.ProviderJianPay, order.OutTradeNo)
	require.NoError(t, err)
	require.Len(t, providers, 1)
	n := jianPayFixtureNotification(order)
	require.NoError(t, svc.ConfirmNotificationUpstream(ctx, providers[0], n))
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderJianPay))
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderJianPay))
	require.Equal(t, order.Amount, *credited)
	_, err = svc.GetWebhookProviders(ctx, payment.ProviderJianPay, "unknown-order")
	require.Error(t, err)
}

func TestJianPayRejectsMismatchedPayment(t *testing.T) {
	for _, mismatch := range []string{"merchant", "order", "method", "amount", "missing", "provider", "trade"} {
		t.Run(mismatch, func(t *testing.T) {
			svc, order, _, credited, _ := newJianPayFixture(t)
			n := jianPayFixtureNotification(order)
			providerKey := payment.ProviderJianPay
			switch mismatch {
			case "merchant":
				n.Metadata["clientNo"] = "other"
			case "order":
				n.Metadata["merchantOrderNo"] = "other"
			case "method":
				n.Metadata["payMethod"] = "wx"
			case "amount":
				n.Metadata["amount_cents"] = "7999"
				n.Amount = 79.99
			case "missing":
				n.Metadata = nil
			case "provider":
				providerKey = payment.ProviderZhifuFM
			case "trade":
				n.TradeNo = ""
			}
			require.Error(t, svc.HandlePaymentNotification(context.Background(), n, providerKey))
			require.Zero(t, *credited)
			fresh, err := svc.entClient.PaymentOrder.Get(context.Background(), order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusPending, fresh.Status)
		})
	}
}

func TestJianPayReconciliationUsesSavedPlatformOrder(t *testing.T) {
	svc, order, _, credited, calls := newJianPayFixture(t)
	recovered, err := svc.ReconcilePendingPaymentOrders(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, order.Amount, *credited)
	recovered, err = svc.ReconcilePendingPaymentOrders(context.Background())
	require.NoError(t, err)
	require.Zero(t, recovered)
	require.Equal(t, order.Amount, *credited)
}

func TestJianPayPlatformQueryKeepsMerchantOrderBinding(t *testing.T) {
	svc, order, _, credited, _ := newJianPayFixture(t)
	ctx := context.Background()
	_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetOutTradeNo("different-local-order").Save(ctx)
	require.NoError(t, err)
	verified, err := svc.VerifyOrderByOutTradeNo(ctx, "different-local-order", order.UserID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, verified.Status)
	require.Zero(t, *credited)
}

func TestJianPayCallbackRejectsDifferentSavedPlatformOrder(t *testing.T) {
	svc, order, _, credited, calls := newJianPayFixture(t)
	ctx := context.Background()
	prov, err := svc.GetWebhookProvider(ctx, payment.ProviderJianPay, order.OutTradeNo)
	require.NoError(t, err)
	n := jianPayFixtureNotification(order)
	n.TradeNo = "other-platform-order"
	require.ErrorContains(t, svc.ConfirmNotificationUpstream(ctx, prov, n), "upstream order number mismatch")
	require.Zero(t, calls.Load())
	require.Zero(t, *credited)
}

func TestJianPayMissingPlatformOrderWaitsForVerifiedCallback(t *testing.T) {
	svc, order, _, credited, calls := newJianPayFixture(t)
	ctx := context.Background()
	order, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetPaymentTradeNo("").Save(ctx)
	require.NoError(t, err)
	verified, err := svc.VerifyOrderByOutTradeNo(ctx, order.OutTradeNo, order.UserID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, verified.Status)
	require.Zero(t, calls.Load())
	require.Zero(t, *credited)
	prov, err := svc.GetWebhookProvider(ctx, payment.ProviderJianPay, order.OutTradeNo)
	require.NoError(t, err)
	n := jianPayFixtureNotification(order)
	require.NoError(t, svc.ConfirmNotificationUpstream(ctx, prov, n))
	require.NoError(t, svc.HandlePaymentNotification(ctx, n, payment.ProviderJianPay))
	require.Equal(t, order.Amount, *credited)
	fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, "jian-trade", fresh.PaymentTradeNo)
}

func TestJianPayRoutingAndCredentialProtection(t *testing.T) {
	svc, order, inst, _, _ := newJianPayFixture(t)
	ctx := context.Background()
	for _, config := range []map[string]string{{"clientNo": "other"}, {"merchantKey": "other"}, {"apiBase": "https://other.test"}} {
		_, err := svc.configService.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{Config: config})
		require.Error(t, err)
	}
	require.Error(t, svc.configService.DeleteProviderInstance(ctx, inst.ID))
	_, err := svc.configService.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{RefundEnabled: boolPtr(true)})
	require.Error(t, err)
	masked, err := svc.configService.decryptAndMaskConfig(payment.ProviderJianPay, inst.Config)
	require.NoError(t, err)
	require.NotContains(t, masked, "merchantKey")
	require.Equal(t, "JP-test", masked["clientNo"])
	require.NotContains(t, order.ProviderSnapshot, "merchantKey")
	require.Equal(t, order.PaymentTradeNo, paymentOrderQueryReference(order, nil))
	require.NoError(t, validateProviderRequest(payment.ProviderJianPay, "简付", "alipay,wxpay"))
	require.Error(t, validateProviderRequest(payment.ProviderJianPay, "简付", "stripe"))
	repo := &paymentConfigSettingRepoStub{values: map[string]string{SettingPaymentVisibleMethodAlipaySource: VisibleMethodSourceJianPayAlipay, SettingPaymentVisibleMethodWxpaySource: VisibleMethodSourceJianPayWechat}}
	svc.configService.settingRepo = repo
	selected, err := svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.NoError(t, err)
	require.Equal(t, inst.ID, selected.ID)
	_, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeWxpay)
	require.Error(t, err, "WeChat is disabled on this instance")
	// Adding JianPay must not implicitly redirect existing unconfigured methods.
	delete(repo.values, SettingPaymentVisibleMethodAlipaySource)
	_, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.Error(t, err)
	_, err = svc.entClient.PaymentProviderInstance.Create().SetProviderKey(payment.TypeEasyPay).SetName("existing").SetConfig("{}").SetSupportedTypes("alipay").Save(ctx)
	require.NoError(t, err)
	selected, err = svc.configService.resolveEnabledVisibleMethodInstance(ctx, payment.TypeAlipay)
	require.NoError(t, err)
	require.Equal(t, payment.TypeEasyPay, selected.ProviderKey)
}

func TestJianPayLatePaymentRequiresReview(t *testing.T) {
	svc, order, _, credited, _ := newJianPayFixture(t)
	ctx := context.Background()
	_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusExpired).SetUpdatedAt(time.Now().Add(-time.Hour)).Save(ctx)
	require.NoError(t, err)
	require.Error(t, svc.HandlePaymentNotification(ctx, jianPayFixtureNotification(order), payment.ProviderJianPay))
	require.Zero(t, *credited)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "PAYMENT_AFTER_EXPIRY"))
}

func TestJianPayConfigurationEncryptsAndPreservesSecret(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewPaymentConfigService(client, nil, []byte(strings.Repeat("k", 32)))
	inst, err := svc.CreateProviderInstance(ctx, CreateProviderInstanceRequest{ProviderKey: payment.ProviderJianPay, Name: "简付", SupportedTypes: []string{payment.TypeAlipay}, Enabled: true, Config: map[string]string{"clientNo": "JP-test", "merchantKey": "fixture-key", "notifyUrl": "https://site.test/api/v1/payment/webhook/jianpay", "returnUrl": "https://site.test/payment/result"}})
	require.NoError(t, err)
	require.NotContains(t, inst.Config, "fixture-key")
	_, err = svc.UpdateProviderInstance(ctx, inst.ID, UpdateProviderInstanceRequest{Config: map[string]string{"merchantKey": ""}})
	require.NoError(t, err)
	stored, err := client.PaymentProviderInstance.Get(ctx, inst.ID)
	require.NoError(t, err)
	decrypted, err := svc.decryptConfig(stored.Config)
	require.NoError(t, err)
	require.Equal(t, "fixture-key", decrypted["merchantKey"])
	visible, err := svc.ListProviderInstancesWithConfig(ctx)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.NotContains(t, visible[0].Config, "merchantKey")
}
