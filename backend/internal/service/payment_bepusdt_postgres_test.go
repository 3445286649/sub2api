//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func bepusdtPostgresClient(t *testing.T) *dbent.Client {
	t.Helper()
	dsn := os.Getenv("SUBAPI_BEPUSDT_TEST_DSN")
	if dsn == "" {
		t.Skip("SUBAPI_BEPUSDT_TEST_DSN is not configured")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, u.Path, "fixture", "integration tests require a dedicated fixture database")
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	schema := "bep_fixture_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = db.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = db.Close() })
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	schemaDB, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, schemaDB)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(context.Background()))
	_, err = client.ExecContext(context.Background(), `CREATE TABLE payment_callback_nonces (id BIGSERIAL PRIMARY KEY, provider_instance_id VARCHAR(64) NOT NULL,key_id VARCHAR(64) NOT NULL,nonce VARCHAR(128) NOT NULL,expires_at TIMESTAMPTZ NOT NULL,UNIQUE(provider_instance_id,key_id,nonce))`)
	require.NoError(t, err)
	return client
}

func bepusdtPostgresSeed(t *testing.T, client *dbent.Client) (*PaymentService, *dbent.PaymentProviderInstance, *User, CreateOrderRequest, *payment.InstanceSelection) {
	t.Helper()
	ctx := context.Background()
	cfg := map[string]string{"apiUrl": "https://gateway.example", "merchantId": "subapi", "keyId": "key", "apiSecret": strings.Repeat("a", 32), "callbackSecret": strings.Repeat("b", 32), "allowedNetworks": "bsc_mainnet,tron_mainnet"}
	raw, _ := json.Marshal(cfg)
	inst, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.ProviderBepusdt).SetName("gateway").SetConfig(string(raw)).SetEnabled(true).SetSupportedTypes(payment.TypeUSDTGateway).Save(ctx)
	require.NoError(t, err)
	storedUser, err := client.User.Create().SetEmail(uuid.NewString() + "@example.com").SetUsername("fixture").SetPasswordHash("h").Save(ctx)
	require.NoError(t, err)
	user := &User{ID: storedUser.ID, Email: storedUser.Email, Username: storedUser.Username}
	req := CreateOrderRequest{UserID: user.ID, Amount: 7, PaymentType: payment.TypeUSDTGateway, CryptoNetwork: "bsc_mainnet", ClientRequestID: uuid.NewString(), OrderType: payment.OrderTypeBalance}
	sel := &payment.InstanceSelection{InstanceID: strconv.FormatInt(inst.ID, 10), ProviderKey: payment.ProviderBepusdt, Config: cfg, SupportedTypes: payment.TypeUSDTGateway}
	svc := &PaymentService{entClient: client, configService: NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil)}
	return svc, inst, user, req, sel
}

func TestBepusdtPostgresDisableSerializesOrderCreation(t *testing.T) {
	client := bepusdtPostgresClient(t)
	svc, inst, user, req, sel := bepusdtPostgresSeed(t, client)
	ctx := context.Background()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	require.NoError(t, lockCryptoProviderInstance(ctx, tx.Client(), inst.ID))
	_, err = tx.PaymentProviderInstance.UpdateOneID(inst.ID).SetEnabled(false).Save(ctx)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, err := svc.createOrderInTx(ctx, req, user, nil, &PaymentConfig{MaxPendingOrders: 10, OrderTimeoutMin: 20}, 7, 7, 0, 7, 0, sel)
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("create bypassed locked provider row: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, tx.Commit())
	require.ErrorContains(t, <-result, "no longer accepts")
	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestBepusdtPostgresIntentUniqueAndCryptoChannelLock(t *testing.T) {
	client := bepusdtPostgresClient(t)
	svc, inst, user, req, sel := bepusdtPostgresSeed(t, client)
	ctx := context.Background()
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.createOrderInTx(ctx, req, user, nil, &PaymentConfig{MaxPendingOrders: 10, OrderTimeoutMin: 20}, 7, 7, 0, 7, 0, sel)
			results <- err
		}()
	}
	first, second := <-results, <-results
	require.True(t, (first == nil) != (second == nil), "exactly one intent creates an order: %v / %v", first, second)
	count, err := client.PaymentOrder.Query().Where(paymentorder.ClientRequestIDEQ(req.ClientRequestID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	gwOrder, err := client.PaymentOrder.Query().Where(paymentorder.ClientRequestIDEQ(req.ClientRequestID)).Only(ctx)
	require.NoError(t, err)
	createInput, err := bepusdtCreateRequest(gwOrder)
	require.NoError(t, err)
	validQuote := provider.BepusdtOrder{ProtocolVersion: "2", MerchantID: "subapi", OutTradeNo: gwOrder.OutTradeNo, GatewayOrderID: "gw_enable_fixture", RequestDigest: provider.BepusdtRequestDigest("subapi", createInput), OrderVersion: 1, Status: "WAITING", FiatAmount: "7.00", FiatCurrency: "CNY", Asset: "USDT", Network: "bsc_mainnet", TokenContract: "0x55d398326f99059ff775485246999027b3197955", TokenDecimals: 18, AmountAtomic: "1000123000000000000", CryptoAmount: "1.000123", ReceiveAddress: "0x1111111111111111111111111111111111111111", LockedRate: "7", Rounding: "ceil_6dp", QuoteVersion: 1, CreatedAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339), ExpiresAt: createInput.ExpiresAt, RequiredConfirmations: 20}
	require.NoError(t, svc.persistBepusdtOrder(ctx, gwOrder, &validQuote))
	// A different provider waits on the shared configuration lock, then observes
	// the first channel's committed enablement instead of enabling concurrently.
	legacy, err := client.PaymentProviderInstance.Create().SetProviderKey(payment.TypeUSDTBSC).SetName("legacy").SetConfig(`{"receiveAddress":"0x1111111111111111111111111111111111111111","cnyPerUsdt":"7"}`).SetEnabled(false).SetSupportedTypes(payment.TypeUSDTBSC).Save(ctx)
	require.NoError(t, err)
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	_, err = tx.Client().ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(738294016))
	require.NoError(t, err)
	blocked := make(chan error, 1)
	enabled := true
	go func() {
		_, err := svc.configService.UpdateProviderInstance(ctx, legacy.ID, UpdateProviderInstanceRequest{Enabled: &enabled})
		blocked <- err
	}()
	select {
	case err := <-blocked:
		t.Fatalf("enablement bypassed shared lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, tx.Commit())
	require.ErrorContains(t, <-blocked, "simultaneously")
	fresh, err := client.PaymentProviderInstance.Get(ctx, inst.ID)
	require.NoError(t, err)
	require.True(t, fresh.Enabled)
}

func TestBepusdtPostgresReceiptRollbackAndCrossOrderUnique(t *testing.T) {
	client := bepusdtPostgresClient(t)
	svc, _, user, req, sel := bepusdtPostgresSeed(t, client)
	ctx := context.Background()
	order, err := svc.createOrderInTx(ctx, req, user, nil, &PaymentConfig{MaxPendingOrders: 10, OrderTimeoutMin: 20}, 7, 7, 0, 7, 0, sel)
	require.NoError(t, err)
	input, err := bepusdtCreateRequest(order)
	require.NoError(t, err)
	quote := provider.BepusdtOrder{ProtocolVersion: "2", MerchantID: "subapi", OutTradeNo: order.OutTradeNo, GatewayOrderID: "gw_fixture", RequestDigest: provider.BepusdtRequestDigest("subapi", input), OrderVersion: 1, Status: "WAITING", FiatAmount: "7.00", FiatCurrency: "CNY", Asset: "USDT", Network: "bsc_mainnet", TokenContract: "0x55d398326f99059ff775485246999027b3197955", TokenDecimals: 18, AmountAtomic: "1000123000000000000", CryptoAmount: "1.000123", ReceiveAddress: "0x1111111111111111111111111111111111111111", LockedRate: "7", Rounding: "ceil_6dp", QuoteVersion: 1, CreatedAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339), ExpiresAt: input.ExpiresAt, RequiredConfirmations: 20}
	quote = paidBepusdtQuote(quote)
	_, err = client.ExecContext(ctx, `CREATE FUNCTION fixture_reject_paid_audit() RETURNS trigger AS $$ BEGIN IF NEW.action='ORDER_PAID' THEN RAISE EXCEPTION 'fixture audit rejection'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `CREATE TRIGGER fixture_audit_failure BEFORE INSERT ON payment_audit_logs FOR EACH ROW EXECUTE FUNCTION fixture_reject_paid_audit()`)
	require.NoError(t, err)
	require.Error(t, svc.acceptBepusdtQuote(ctx, order, &quote))
	count, err := client.CryptoPaymentReceipt.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	fresh, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.PaidAt)
	_, err = client.ExecContext(ctx, "DROP TRIGGER fixture_audit_failure ON payment_audit_logs")
	require.NoError(t, err)
	// Reuse the idempotent redeem behavior while the acceptance transaction uses
	// actual PostgreSQL constraints and audit failure injection.
	userRepo := &mockUserRepo{getByIDUser: user}
	userRepo.updateBalanceFn = func(context.Context, int64, float64) error { return nil }
	repo := &paymentOrderLifecycleRedeemRepo{codesByCode: map[string]*RedeemCode{order.RechargeCode: {ID: 1, Code: order.RechargeCode, Type: RedeemTypeBalance, Value: 7, Status: StatusUnused}}}
	svc.userRepo = userRepo
	svc.redeemService = NewRedeemService(repo, userRepo, nil, nil, nil, client, nil, nil)
	require.NoError(t, svc.acceptBepusdtQuote(ctx, order, &quote))
	require.NoError(t, svc.acceptBepusdtQuote(ctx, order, &quote))
	count, err = client.CryptoPaymentReceipt.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	request2 := req
	request2.ClientRequestID = uuid.NewString()
	order2, err := svc.createOrderInTx(ctx, request2, user, nil, &PaymentConfig{MaxPendingOrders: 10, OrderTimeoutMin: 20}, 7, 7, 0, 7, 0, sel)
	require.NoError(t, err)
	input2, err := bepusdtCreateRequest(order2)
	require.NoError(t, err)
	quote.OutTradeNo = order2.OutTradeNo
	quote.RequestDigest = provider.BepusdtRequestDigest("subapi", input2)
	quote.ExpiresAt = input2.ExpiresAt
	quote.GatewayOrderID = "gw_second"
	require.ErrorContains(t, svc.acceptBepusdtQuote(ctx, order2, &quote), "another order")
	fresh, err = client.PaymentOrder.Get(ctx, order2.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.PaidAt)
	t.Log(fmt.Sprintf("receipt rows=%d", count))
}
