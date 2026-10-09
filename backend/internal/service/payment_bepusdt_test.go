//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptopaymentreceipt"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func bepusdtTestSetup(t *testing.T, ctx context.Context, apiURL string) (*PaymentService, *dbent.PaymentOrder, provider.BepusdtOrder, CreateOrderRequest) {
	t.Helper()
	client := newPaymentOrderLifecycleTestClient(t)
	_, nonceErr := client.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS payment_callback_nonces(id INTEGER PRIMARY KEY, provider_instance_id TEXT NOT NULL,key_id TEXT NOT NULL,nonce TEXT NOT NULL,expires_at DATETIME NOT NULL,UNIQUE(provider_instance_id,key_id,nonce))")
	require.NoError(t, nonceErr)
	cfg := map[string]string{"apiUrl": apiURL, "merchantId": "subapi", "keyId": "key", "apiSecret": strings.Repeat("a", 32), "callbackSecret": strings.Repeat("b", 32), "allowedNetworks": "bsc_mainnet,tron_mainnet"}
	raw, _ := json.Marshal(cfg)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.ProviderBepusdt).SetName("gateway").SetConfig(string(raw)).SetEnabled(true).SetSupportedTypes(payment.TypeUSDTGateway).Save(ctx)
	require.NoError(t, err)
	user, err := client.User.Create().SetEmail(uuid.NewString() + "@example.com").SetUsername("u").SetPasswordHash("h").Save(ctx)
	require.NoError(t, err)
	request := CreateOrderRequest{UserID: user.ID, Amount: 7, PaymentType: payment.TypeUSDTGateway, CryptoNetwork: "bsc_mainnet", OrderType: payment.OrderTypeBalance, ClientRequestID: uuid.NewString()}
	selection := &payment.InstanceSelection{InstanceID: strconv.FormatInt(inst.ID, 10), ProviderKey: inst.ProviderKey, Config: cfg}
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	snapshot := buildBepusdtInitialSnapshot(selection, request, "sub2_gateway_"+uuid.NewString(), 7, expires)
	input := snapshot[payment.ProviderBepusdt].(map[string]any)["create_request"].(provider.BepusdtCreateRequest)
	order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).SetAmount(7).SetPayAmount(7).SetRechargeCode("PAY-" + uuid.NewString()).SetOutTradeNo(input.OutTradeNo).SetPaymentType(payment.TypeUSDTGateway).SetProviderKey(payment.ProviderBepusdt).SetProviderInstanceID(selection.InstanceID).SetClientRequestID(request.ClientRequestID).SetProviderSnapshot(snapshot).SetPaymentTradeNo("").SetStatus(OrderStatusPending).SetExpiresAt(expires).SetClientIP("127.0.0.1").SetSrcHost("example.com").Save(ctx)
	require.NoError(t, err)
	quote := provider.BepusdtOrder{ProtocolVersion: "2", MerchantID: "subapi", OutTradeNo: input.OutTradeNo, GatewayOrderID: "gw_" + uuid.NewString(), RequestDigest: provider.BepusdtRequestDigest("subapi", input), OrderVersion: 1, Status: "WAITING", FiatAmount: "7.00", FiatCurrency: "CNY", Asset: "USDT", Network: request.CryptoNetwork, TokenContract: "0x55d398326f99059ff775485246999027b3197955", TokenDecimals: 18, AmountAtomic: "1000123000000000000", CryptoAmount: "1.000123", ReceiveAddress: "0x1111111111111111111111111111111111111111", LockedRate: "7", Rounding: "ceil_6dp", QuoteVersion: 1, CreatedAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339), ExpiresAt: input.ExpiresAt, RequiredConfirmations: 20}
	userRepo := &mockUserRepo{getByIDUser: &User{ID: user.ID, Email: user.Email, Username: user.Username}}
	userRepo.updateBalanceFn = func(_ context.Context, _ int64, value float64) error {
		userRepo.getByIDUser.Balance += value
		return nil
	}
	redeemRepo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{order.RechargeCode: {ID: 1, Code: order.RechargeCode, Type: RedeemTypeBalance, Value: 7, Status: StatusUnused}}}
	service := &PaymentService{entClient: client, configService: NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil), loadBalancer: payment.NewDefaultLoadBalancer(client, nil), registry: payment.NewRegistry(), redeemService: NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil), userRepo: userRepo}
	return service, order, quote, request
}
func paidBepusdtQuote(quote provider.BepusdtOrder) provider.BepusdtOrder {
	quote.Status = "PAID"
	quote.OrderVersion = 2
	quote.Payment = &provider.BepusdtPayment{TxHash: "0x" + strings.Repeat("b", 64), EventIndex: 0, BlockNumber: 100, BlockHash: "0x" + strings.Repeat("c", 64), BlockTime: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), ToAddress: quote.ReceiveAddress, AmountAtomic: quote.AmountAtomic, Confirmations: 20, Confirmed: true}
	return quote
}

func TestBepusdtClientIntentAndUnknownCreationRecovery(t *testing.T) {
	ctx := context.Background()
	var quote provider.BepusdtOrder
	posted := false
	queryCount := 0
	createCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			queryCount++
			if !posted {
				w.WriteHeader(404)
				_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND"}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(quote)
			return
		}
		createCount++
		body, _ := io.ReadAll(r.Body)
		var input provider.BepusdtCreateRequest
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, quote.OutTradeNo, input.OutTradeNo)
		posted = true
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":{"code":"RESPONSE_LOST"}}`))
	}))
	defer server.Close()
	svc, order, fixture, request := bepusdtTestSetup(t, ctx, server.URL)
	quote = fixture
	resp, err := svc.resumeBepusdtCreation(ctx, order)
	require.NoError(t, err)
	require.Equal(t, "unknown", resp.CreateStage)
	require.Equal(t, order.ID, resp.OrderID)
	fresh, err := svc.findBepusdtRequest(ctx, request)
	require.NoError(t, err)
	require.Equal(t, order.ID, fresh.ID)
	request.Amount = 8
	_, err = svc.findBepusdtRequest(ctx, request)
	require.ErrorContains(t, err, "different payment intent")
	// A later retry queries the existing gateway intent instead of issuing a second create.
	next := cloneProviderSnapshot(fresh.ProviderSnapshot)
	nested := next[payment.ProviderBepusdt].(map[string]any)
	delete(nested, "retry_after")
	fresh, err = svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(next).Save(ctx)
	require.NoError(t, err)
	resp, err = svc.resumeBepusdtCreation(ctx, fresh)
	require.NoError(t, err)
	require.Equal(t, "ready", resp.CreateStage)
	require.Equal(t, 1, createCount)
	require.Equal(t, 2, queryCount)
	require.Equal(t, quote.ReceiveAddress, resp.ReceiveAddress)
}
func TestBepusdtSharedReceiptAndImmutableQuote(t *testing.T) {
	ctx := context.Background()
	svc, order, quote, _ := bepusdtTestSetup(t, ctx, "https://gateway.example")
	require.NoError(t, svc.persistBepusdtOrder(ctx, order, &quote))
	fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	changed := quote
	changed.CryptoAmount = "1.000124"
	changed.AmountAtomic = "1000124000000000000"
	require.ErrorContains(t, svc.persistBepusdtOrder(ctx, fresh, &changed), "immutable quote changed")
	paid := paidBepusdtQuote(quote)
	require.NoError(t, svc.acceptBepusdtQuote(ctx, fresh, &paid))
	require.NoError(t, svc.acceptBepusdtQuote(ctx, fresh, &paid))
	count, err := svc.entClient.CryptoPaymentReceipt.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 7.0, svc.userRepo.(*mockUserRepo).getByIDUser.Balance)
	// Legacy BSC cannot consume a transaction already accepted by the new gateway.
	legacy, err := svc.entClient.PaymentOrder.Create().SetUserID(order.UserID).SetUserEmail(order.UserEmail).SetUserName(order.UserName).SetAmount(7).SetPayAmount(7).SetRechargeCode("legacy").SetOutTradeNo("sub2_legacy_" + uuid.NewString()).SetPaymentType(payment.TypeUSDTBSC).SetPaymentTradeNo("").SetStatus(OrderStatusPending).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("example.com").Save(ctx)
	require.NoError(t, err)
	require.ErrorContains(t, svc.acceptCryptoPayment(ctx, legacy, paid.Payment.TxHash, 7, payment.TypeUSDTBSC, nil), "another order")
	loaded, err := svc.entClient.PaymentOrder.Get(ctx, legacy.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.PaidAt)
	proof, err := svc.entClient.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.OrderIDEQ(order.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "bsc_mainnet", proof.ChainID)
}
func TestBepusdtDisabledProviderHistoryAndQuoteSurvive(t *testing.T) {
	ctx := context.Background()
	svc, order, quote, _ := bepusdtTestSetup(t, ctx, "https://gateway.example")
	id, _ := strconv.ParseInt(*order.ProviderInstanceID, 10, 64)
	disabled := false
	inst, err := svc.configService.UpdateProviderInstance(ctx, id, UpdateProviderInstanceRequest{Enabled: &disabled})
	require.NoError(t, err)
	require.False(t, inst.Enabled)
	_, err = svc.configService.UpdateProviderInstance(ctx, id, UpdateProviderInstanceRequest{Config: map[string]string{"apiUrl": "https://different.example"}})
	require.ErrorContains(t, err, "frozen")
	require.ErrorContains(t, svc.configService.DeleteProviderInstance(ctx, id), "historical orders")
	require.NoError(t, svc.persistBepusdtOrder(ctx, order, &quote))
	prov, err := svc.getOrderProvider(ctx, order)
	require.NoError(t, err)
	require.Equal(t, payment.ProviderBepusdt, prov.ProviderKey())
}

func TestBepusdtPaidTransactionRollsBackReceiptWithAudit(t *testing.T) {
	ctx := context.Background()
	svc, order, quote, _ := bepusdtTestSetup(t, ctx, "https://gateway.example")
	paid := paidBepusdtQuote(quote)
	_, err := svc.entClient.ExecContext(ctx, "CREATE TRIGGER fail_paid_audit BEFORE INSERT ON payment_audit_logs WHEN NEW.action='ORDER_PAID' BEGIN SELECT RAISE(ABORT,'fixture audit rejected'); END")
	require.NoError(t, err)
	require.Error(t, svc.acceptBepusdtQuote(ctx, order, &paid))
	fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.PaidAt)
	require.Equal(t, OrderStatusPending, fresh.Status)
	count, err := svc.entClient.CryptoPaymentReceipt.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Zero(t, svc.userRepo.(*mockUserRepo).getByIDUser.Balance)
}

func TestBepusdtCallbackQueriesDisabledInstanceAndAcceptsOnlyPaid(t *testing.T) {
	ctx := context.Background()
	var quote provider.BepusdtOrder
	queries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		queries++
		_ = json.NewEncoder(w).Encode(quote)
	}))
	defer server.Close()
	svc, order, fixture, _ := bepusdtTestSetup(t, ctx, server.URL)
	quote = paidBepusdtQuote(fixture)
	id, _ := strconv.ParseInt(*order.ProviderInstanceID, 10, 64)
	_, err := svc.entClient.PaymentProviderInstance.UpdateOneID(id).SetEnabled(false).Save(ctx)
	require.NoError(t, err)
	event := provider.BepusdtEvent{EventID: "paid_evt_1", Order: quote}
	raw, _ := json.Marshal(event)
	headers := map[string]string{"x-merchant-id": "subapi", "x-key-id": "key", "x-timestamp": strconv.FormatInt(time.Now().Unix(), 10), "x-nonce": "fixture_nonce_0001", "request-path": provider.BepusdtWebhookPath}
	headers["x-signature"] = provider.BepusdtSignature(strings.Repeat("b", 32), "subapi", "key", "POST", provider.BepusdtWebhookPath, headers["x-timestamp"], headers["x-nonce"], raw)
	// A valid signature is insufficient if the authoritative query is not paid.
	quote.Status = "WAITING"
	_, err = svc.HandleBepusdtEvent(ctx, string(raw), headers)
	require.Error(t, err)
	fresh, err := svc.entClient.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.PaidAt)
	quote.Status = "PAID"
	headers["x-nonce"] = "fixture_nonce_0002"
	headers["x-signature"] = provider.BepusdtSignature(strings.Repeat("b", 32), "subapi", "key", "POST", provider.BepusdtWebhookPath, headers["x-timestamp"], headers["x-nonce"], raw)
	ack, err := svc.HandleBepusdtEvent(ctx, string(raw), headers)
	require.NoError(t, err)
	require.Equal(t, event.EventID, ack)
	require.Equal(t, 2, queries)
	require.Equal(t, 7.0, svc.userRepo.(*mockUserRepo).getByIDUser.Balance)
	_, err = svc.HandleBepusdtEvent(ctx, string(raw), headers)
	require.ErrorContains(t, err, "nonce was already used")
	require.Equal(t, 2, queries)
}

func TestBepusdtEnablementBlocksLegacyBSCButAllowsTron(t *testing.T) {
	ctx := context.Background()
	svc, order, _, _ := bepusdtTestSetup(t, ctx, "https://gateway.example")
	id, _ := strconv.ParseInt(*order.ProviderInstanceID, 10, 64)
	legacy, err := svc.entClient.PaymentProviderInstance.Create().SetProviderKey(payment.TypeUSDTBSC).SetName("legacy").SetConfig(`{"receiveAddress":"0x1111111111111111111111111111111111111111","cnyPerUsdt":"7"}`).SetEnabled(true).SetSupportedTypes(payment.TypeUSDTBSC).Save(ctx)
	require.NoError(t, err)
	require.ErrorContains(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, map[string]string{"allowedNetworks": "bsc_mainnet"}), "simultaneously")
	require.NoError(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, map[string]string{"allowedNetworks": "tron_mainnet"}))
	_, err = svc.entClient.PaymentProviderInstance.UpdateOneID(legacy.ID).SetEnabled(false).Save(ctx)
	require.NoError(t, err)
	_, err = svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetPaymentType(payment.TypeUSDTBSC).SetProviderKey(payment.TypeUSDTBSC).SetPaymentTradeNo("usdt_bsc|legacy|7.00|1.000123|0x1111111111111111111111111111111111111111").Save(ctx)
	require.NoError(t, err)
	require.Error(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, map[string]string{"allowedNetworks": "bsc_mainnet"}))
}

func TestBepusdtLegacyReservationProofAndSubscriptionCurrency(t *testing.T) {
	ctx := context.Background()
	rows := []string{"bsc_mainnet|0x55d398326f99059ff775485246999027b3197955|0x1111111111111111111111111111111111111111|1000123000000000000"}
	proofDigest := legacyBSCReservationDigest(rows)
	healthDigest := legacyBSCReservationDigest(nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/merchant/health", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"protocol_version": "2", "merchant_id": "subapi", "legacy_reservations": map[string]any{"count": 1, "digest": healthDigest}})
	}))
	defer server.Close()
	svc, order, _, _ := bepusdtTestSetup(t, ctx, server.URL)
	id, _ := strconv.ParseInt(*order.ProviderInstanceID, 10, 64)
	inst, err := svc.entClient.PaymentProviderInstance.Get(ctx, id)
	require.NoError(t, err)
	config, err := svc.configService.decryptConfig(inst.Config)
	require.NoError(t, err)
	legacyIntent := "usdt_bsc|legacy|7.00|1.000123|0x1111111111111111111111111111111111111111"
	_, err = svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderKey(payment.TypeUSDTBSC).SetPaymentType(payment.TypeUSDTBSC).SetPaymentTradeNo(legacyIntent).SetProviderSnapshot(map[string]any{"usdt_bsc": map[string]any{"intent_trade_no": legacyIntent, "token_amount": "1.000123"}}).Save(ctx)
	require.NoError(t, err)
	exported, digest, err := svc.configService.LegacyBSCReservationExport(ctx)
	require.NoError(t, err)
	require.Equal(t, rows, exported)
	require.Equal(t, proofDigest, digest)
	require.ErrorContains(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, config), "permanently imported")
	healthDigest = proofDigest
	require.NoError(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, config))
	healthDigest = strings.Repeat("0", 64)
	require.ErrorContains(t, svc.configService.validateCryptoEnablement(ctx, id, payment.ProviderBepusdt, config), "permanently imported")
	price, err := bepusdtSubscriptionCNYPrice(12.34, "USD", 7)
	require.NoError(t, err)
	require.Equal(t, 86.38, price)
	_, err = bepusdtSubscriptionCNYPrice(12.34, "USD", 0)
	require.Error(t, err)
	_, err = bepusdtSubscriptionCNYPrice(12.34, "EUR", 7)
	require.Error(t, err)
}
