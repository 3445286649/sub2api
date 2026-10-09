package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func isCryptoProviderKey(key string) bool {
	return key == payment.TypeUSDTBSC || key == payment.ProviderBepusdt
}

func bepusdtSubscriptionCNYPrice(price float64, currency string, rate float64) (float64, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "CNY" {
		return price, nil
	}
	if currency == "" {
		currency = "USD"
	}
	if currency != "USD" {
		return 0, infraerrors.BadRequest("PLAN_CURRENCY_UNSUPPORTED", "the gateway supports CNY plans or explicitly converted USD plans")
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, infraerrors.BadRequest("SUBSCRIPTION_CONVERSION_REQUIRED", "configure the USD to CNY subscription conversion rate before using this gateway")
	}
	return decimal.NewFromFloat(price).Mul(decimal.NewFromFloat(rate)).Round(2).InexactFloat64(), nil
}
func lockCryptoProviderInstance(ctx context.Context, client *dbent.Client, id int64) error {
	if client.Driver().Dialect() == dialect.Postgres {
		_, err := client.PaymentProviderInstance.Query().Where(paymentproviderinstance.IDEQ(id)).ForUpdate().Only(ctx)
		return err
	}
	_, err := client.ExecContext(ctx, "UPDATE payment_provider_instances SET id=id WHERE id=?", id)
	return err
}
func (s *PaymentConfigService) countProviderHistory(ctx context.Context, id int64) (int, error) {
	return s.entClient.PaymentOrder.Query().Where(paymentorder.ProviderInstanceIDEQ(strconv.FormatInt(id, 10))).Count(ctx)
}

func (s *PaymentConfigService) CheckBepusdtConnection(ctx context.Context, id int64) (map[string]any, error) {
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst.ProviderKey != payment.ProviderBepusdt {
		return nil, infraerrors.BadRequest("INVALID_PROVIDER", "connection check is supported only for bepusdt")
	}
	cfg, err := s.decryptConfig(inst.Config)
	if err != nil {
		return nil, err
	}
	gateway, err := provider.NewBepusdt(strconv.FormatInt(id, 10), cfg)
	if err != nil {
		return nil, err
	}
	return gateway.Health(ctx)
}
func sameCryptoInstanceSelection(current *dbent.PaymentProviderInstance, config map[string]string, selection *payment.InstanceSelection, req CreateOrderRequest) bool {
	selected := map[string]string{}
	for key, value := range selection.Config {
		if key != "paymentMode" {
			selected[key] = value
		}
	}
	if current.ProviderKey != selection.ProviderKey || !reflect.DeepEqual(selected, config) {
		return false
	}
	if !payment.InstanceSupportsType(current.SupportedTypes, req.PaymentType) {
		return false
	}
	if req.PaymentType == payment.TypeUSDTGateway {
		for _, network := range provider.BepusdtNetworks(config) {
			if network == req.CryptoNetwork {
				return true
			}
		}
		return false
	}
	return payment.InstanceSupportsType(current.SupportedTypes, req.PaymentType)
}

func validateBepusdtCreateInput(req CreateOrderRequest) error {
	if req.PaymentType != payment.TypeUSDTGateway {
		if req.CryptoNetwork != "" || req.ClientRequestID != "" {
			return infraerrors.BadRequest("INVALID_CRYPTO_INPUT", "crypto network/request ID are only supported by usdt_gateway")
		}
		return nil
	}
	if req.CryptoNetwork != "bsc_mainnet" && req.CryptoNetwork != "tron_mainnet" {
		return infraerrors.BadRequest("INVALID_CRYPTO_NETWORK", "a supported crypto network is required")
	}
	id, err := uuid.Parse(req.ClientRequestID)
	if err != nil || id.String() != req.ClientRequestID {
		return infraerrors.BadRequest("INVALID_CLIENT_REQUEST_ID", "client_request_id must be a canonical UUID")
	}
	return nil
}
func bepusdtClientIntent(req CreateOrderRequest) string {
	raw, _ := json.Marshal(struct {
		Amount                          string
		PaymentType, OrderType, Network string
		PlanID                          int64
	}{strconv.FormatFloat(req.Amount, 'f', -1, 64), req.PaymentType, req.OrderType, req.CryptoNetwork, req.PlanID})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func (s *PaymentService) findBepusdtRequest(ctx context.Context, req CreateOrderRequest) (*dbent.PaymentOrder, error) {
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(req.UserID), paymentorder.ClientRequestIDEQ(req.ClientRequestID)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snapshot := bepusdtSnapshot(order)
	if snapshot == nil || psSnapshotStringValue(snapshot["client_intent"]) != bepusdtClientIntent(req) {
		return nil, infraerrors.Conflict("CLIENT_REQUEST_CONFLICT", "client_request_id was already used for a different payment intent")
	}
	return order, nil
}
func buildBepusdtInitialSnapshot(sel *payment.InstanceSelection, req CreateOrderRequest, outTradeNo string, payAmount float64, expires time.Time) map[string]any {
	snapshot := buildPaymentOrderProviderSnapshot(sel, req)
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	input := provider.BepusdtCreateRequest{OutTradeNo: outTradeNo, FiatAmount: strconv.FormatFloat(payAmount, 'f', 2, 64), FiatCurrency: "CNY", Asset: "USDT", Network: req.CryptoNetwork, ExpiresAt: expires.UTC().Truncate(time.Second).Format(time.RFC3339)}
	snapshot["currency"] = "CNY"
	snapshot["merchant_id"] = sel.Config["merchantId"]
	snapshot[payment.ProviderBepusdt] = map[string]any{"client_intent": bepusdtClientIntent(req), "request_digest": provider.BepusdtRequestDigest(sel.Config["merchantId"], input), "create_stage": "creating", "create_request": input, "network": req.CryptoNetwork, "merchant_id": sel.Config["merchantId"]}
	return snapshot
}
func bepusdtSnapshot(order *dbent.PaymentOrder) map[string]any {
	if order == nil {
		return nil
	}
	snapshot, _ := order.ProviderSnapshot[payment.ProviderBepusdt].(map[string]any)
	return snapshot
}
func bepusdtCreateRequest(order *dbent.PaymentOrder) (provider.BepusdtCreateRequest, error) {
	var req provider.BepusdtCreateRequest
	snapshot := bepusdtSnapshot(order)
	if snapshot == nil {
		return req, errors.New("missing bepusdt snapshot")
	}
	raw, err := json.Marshal(snapshot["create_request"])
	if err != nil {
		return req, err
	}
	err = json.Unmarshal(raw, &req)
	if err != nil || req.OutTradeNo != order.OutTradeNo {
		return req, errors.New("invalid bepusdt create snapshot")
	}
	return req, nil
}
func bepusdtSnapshotOrder(order *dbent.PaymentOrder) *provider.BepusdtOrder {
	snapshot := bepusdtSnapshot(order)
	if snapshot == nil {
		return nil
	}
	raw, err := json.Marshal(snapshot["order"])
	if err != nil {
		return nil
	}
	var result provider.BepusdtOrder
	if json.Unmarshal(raw, &result) != nil || result.GatewayOrderID == "" {
		return nil
	}
	return &result
}

// BepusdtOrderDisplay projects safe, persisted checkout fields without secrets.
func BepusdtOrderDisplay(order *dbent.PaymentOrder) *CreateOrderResponse {
	snapshot := bepusdtSnapshot(order)
	if snapshot == nil {
		return nil
	}
	result := &CreateOrderResponse{OrderID: order.ID, Amount: order.Amount, PayAmount: order.PayAmount, FeeRate: order.FeeRate, BonusAmount: order.BonusAmount, Status: order.Status, PaymentType: order.PaymentType, OutTradeNo: order.OutTradeNo, Currency: "CNY", CryptoCurrency: "USDT", CryptoNetwork: psSnapshotStringValue(snapshot["network"]), CreateStage: psSnapshotStringValue(snapshot["create_stage"]), ExpiresAt: order.ExpiresAt, ResultType: payment.CreatePaymentResultOrderCreated, PaymentMode: "qrcode"}
	if quote := bepusdtSnapshotOrder(order); quote != nil {
		result.CryptoAmount, result.CryptoNetwork, result.ReceiveAddress, result.QRCode, result.CryptoRate = quote.CryptoAmount, quote.Network, quote.ReceiveAddress, quote.ReceiveAddress, quote.LockedRate
		result.GatewayStatus, result.RequiredConfirmations = quote.Status, quote.RequiredConfirmations
		if quote.Payment != nil {
			result.Confirmations = quote.Payment.Confirmations
		}
	}
	return result
}

func BepusdtPlanCurrency(order *dbent.PaymentOrder) string {
	return psSnapshotStringValue(bepusdtSnapshot(order)["plan_currency"])
}

func (s *PaymentService) persistBepusdtOrder(ctx context.Context, order *dbent.PaymentOrder, quote *provider.BepusdtOrder) error {
	if err := s.validateBepusdtQuote(order, quote); err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockCryptoPaymentOrder(ctx, tx.Client(), order.ID); err != nil {
		return err
	}
	current, err := tx.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		return err
	}
	if err = s.validateBepusdtQuote(current, quote); err != nil {
		return err
	}
	if existing := bepusdtSnapshotOrder(current); existing != nil && existing.OrderVersion > quote.OrderVersion {
		return nil
	}
	if current.PaidAt != nil && quote.Status != "PAID" {
		return nil
	}
	next := cloneProviderSnapshot(current.ProviderSnapshot)
	nested := next[payment.ProviderBepusdt].(map[string]any)
	nested["order"] = quote
	nested["create_stage"] = "ready"
	delete(nested, "retry_after")
	update := tx.PaymentOrder.UpdateOneID(current.ID).SetProviderSnapshot(next).SetQrCode(quote.ReceiveAddress)
	if current.Status == OrderStatusPending && current.PaidAt == nil {
		if quote.Status == "EXPIRED" {
			update.SetStatus(OrderStatusExpired)
		}
		if quote.Status == "CANCELLED" {
			update.SetStatus(OrderStatusCancelled)
		}
	}
	_, err = update.Save(ctx)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *PaymentService) validateBepusdtQuote(order *dbent.PaymentOrder, quote *provider.BepusdtOrder) error {
	if err := provider.ValidateBepusdtOrder(quote); err != nil {
		return err
	}
	req, err := bepusdtCreateRequest(order)
	if err != nil {
		return err
	}
	if quote.OutTradeNo != order.OutTradeNo || quote.RequestDigest != psSnapshotStringValue(bepusdtSnapshot(order)["request_digest"]) || quote.Network != req.Network || quote.FiatAmount != req.FiatAmount || quote.FiatCurrency != req.FiatCurrency || quote.ExpiresAt != req.ExpiresAt || quote.MerchantID != psSnapshotStringValue(bepusdtSnapshot(order)["merchant_id"]) {
		return errors.New("bepusdt order does not match local immutable intent")
	}
	if previous := bepusdtSnapshotOrder(order); previous != nil {
		if quote.GatewayOrderID != previous.GatewayOrderID || quote.AmountAtomic != previous.AmountAtomic || quote.ReceiveAddress != previous.ReceiveAddress || quote.TokenContract != previous.TokenContract || quote.LockedRate != previous.LockedRate || quote.CreatedAt != previous.CreatedAt || quote.RequiredConfirmations != previous.RequiredConfirmations {
			return errors.New("bepusdt immutable quote changed")
		}
		if previous.Status == "PAID" && previous.Payment != nil && !bepusdtSamePaymentIdentity(previous.Payment, quote.Payment) {
			return errors.New("bepusdt paid receipt identity changed")
		}
	}
	return nil
}

func bepusdtSamePaymentIdentity(a, b *provider.BepusdtPayment) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.TxHash, b.TxHash) && a.EventIndex == b.EventIndex && a.BlockNumber == b.BlockNumber && strings.EqualFold(a.BlockHash, b.BlockHash) && a.BlockTime == b.BlockTime && a.FromAddress == b.FromAddress && a.ToAddress == b.ToAddress && a.AmountAtomic == b.AmountAtomic
}
func (s *PaymentService) markBepusdtCreation(ctx context.Context, orderID int64, stage string) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockCryptoPaymentOrder(ctx, tx.Client(), orderID); err != nil {
		return err
	}
	order, err := tx.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return err
	}
	if order.PaidAt != nil || bepusdtSnapshotOrder(order) != nil {
		return nil
	}
	next := cloneProviderSnapshot(order.ProviderSnapshot)
	nested := next[payment.ProviderBepusdt].(map[string]any)
	nested["create_stage"] = stage
	attempts := psSnapshotIntValue(nested["create_attempts"]) + 1
	nested["create_attempts"] = attempts
	delay := time.Duration(1<<min(attempts, 8)) * time.Second
	nested["retry_after"] = time.Now().Add(delay).UTC().Format(time.RFC3339)
	update := tx.PaymentOrder.UpdateOneID(orderID).SetProviderSnapshot(next)
	if stage == "rejected" {
		update.SetStatus(OrderStatusFailed).SetFailedAt(time.Now()).SetFailedReason("gateway rejected payment creation")
	}
	if _, err = update.Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *PaymentService) resumeBepusdtCreation(ctx context.Context, order *dbent.PaymentOrder) (*CreateOrderResponse, error) {
	if order.PaidAt != nil || order.Status != OrderStatusPending {
		return BepusdtOrderDisplay(order), nil
	}
	if quote := bepusdtSnapshotOrder(order); quote != nil {
		return BepusdtOrderDisplay(order), nil
	}
	if retryAfter, err := time.Parse(time.RFC3339, psSnapshotStringValue(bepusdtSnapshot(order)["retry_after"])); err == nil && time.Now().Before(retryAfter) {
		return BepusdtOrderDisplay(order), nil
	}
	prov, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return nil, err
	}
	gateway, ok := prov.(*provider.Bepusdt)
	if !ok {
		return nil, errors.New("order provider is not bepusdt")
	}
	quote, err := gateway.Query(ctx, order.OutTradeNo)
	var upstream *provider.BepusdtAPIError
	creationRejected := false
	if errors.As(err, &upstream) && upstream.Status == 404 {
		req, e := bepusdtCreateRequest(order)
		if e != nil {
			return nil, e
		}
		created, e := gateway.CreatePayment(ctx, payment.CreatePaymentRequest{OrderID: req.OutTradeNo, Amount: req.FiatAmount, CryptoNetwork: req.Network, ExpiresAt: req.ExpiresAt})
		if e == nil {
			var decoded provider.BepusdtOrder
			e = json.Unmarshal([]byte(created.Metadata["bepusdt_order"]), &decoded)
			quote = &decoded
		}
		err = e
		creationRejected = provider.BepusdtDefinitiveError(e)
	}
	if err != nil {
		stage := "unknown"
		if creationRejected {
			stage = "rejected"
		}
		if e := s.markBepusdtCreation(ctx, order.ID, stage); e != nil {
			return nil, e
		}
		fresh, e := s.entClient.PaymentOrder.Get(ctx, order.ID)
		if e != nil {
			return nil, e
		}
		return BepusdtOrderDisplay(fresh), nil
	}
	if err = s.persistBepusdtOrder(ctx, order, quote); err != nil {
		return nil, err
	}
	if quote.Status == "PAID" {
		if err = s.acceptBepusdtQuote(ctx, order, quote); err != nil {
			return nil, err
		}
	}
	fresh, err := s.entClient.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	return BepusdtOrderDisplay(fresh), nil
}

func (s *PaymentService) HandleBepusdtEvent(ctx context.Context, raw string, headers map[string]string) (string, error) {
	var untrusted provider.BepusdtEvent
	if json.Unmarshal([]byte(raw), &untrusted) != nil || untrusted.Order.OutTradeNo == "" {
		return "", infraerrors.BadRequest("INVALID_CALLBACK", "invalid gateway callback")
	}
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.OutTradeNoEQ(untrusted.Order.OutTradeNo)).Only(ctx)
	if err != nil {
		return "", err
	}
	if psStringValue(order.ProviderKey) != payment.ProviderBepusdt {
		return "", errors.New("callback order provider mismatch")
	}
	prov, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return "", err
	}
	gateway, ok := prov.(*provider.Bepusdt)
	if !ok {
		return "", errors.New("invalid gateway provider")
	}
	event, err := gateway.VerifyEvent(raw, headers)
	if err != nil {
		return "", err
	}
	if err = s.reserveBepusdtCallbackNonce(ctx, order, headers); err != nil {
		return "", err
	}
	quote, err := gateway.Query(ctx, order.OutTradeNo)
	if err != nil {
		return "", err
	}
	if quote.Status != "PAID" || quote.OrderVersion < event.Order.OrderVersion || quote.GatewayOrderID != event.Order.GatewayOrderID || quote.Payment == nil || event.Order.Payment == nil || !strings.EqualFold(quote.Payment.TxHash, event.Order.Payment.TxHash) || quote.Payment.EventIndex != event.Order.Payment.EventIndex {
		return "", errors.New("callback does not match authoritative paid order")
	}
	if err = s.acceptBepusdtQuote(ctx, order, quote); err != nil {
		s.writeAuditLog(ctx, order.ID, "BEPUSDT_PAYMENT_REJECTED", payment.ProviderBepusdt, map[string]any{"reason": err.Error(), "gateway_order_id": quote.GatewayOrderID})
		s.notifyPaymentReview(ctx, order, "BEPUSDT_PAYMENT_REJECTED", "网关付款证据无法安全接受，需人工复核")
		return "", err
	}
	return event.EventID, nil
}

func (s *PaymentService) reserveBepusdtCallbackNonce(ctx context.Context, order *dbent.PaymentOrder, headers map[string]string) error {
	instance := psStringValue(order.ProviderInstanceID)
	expires := time.Now().Add(10 * time.Minute)
	var query string
	if s.entClient.Driver().Dialect() == dialect.Postgres {
		query = "INSERT INTO payment_callback_nonces(provider_instance_id,key_id,nonce,expires_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING"
	} else {
		query = "INSERT INTO payment_callback_nonces(provider_instance_id,key_id,nonce,expires_at) VALUES(?,?,?,?) ON CONFLICT DO NOTHING"
	}
	result, err := s.entClient.ExecContext(ctx, query, instance, headers["x-key-id"], headers["x-nonce"], expires)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("gateway callback nonce was already used")
	}
	return nil
}
func (s *PaymentService) acceptBepusdtQuote(ctx context.Context, order *dbent.PaymentOrder, quote *provider.BepusdtOrder) error {
	if quote.Status != "PAID" {
		return errors.New("gateway order is not paid")
	}
	if err := s.validateBepusdtQuote(order, quote); err != nil {
		return err
	}
	metadata := map[string]string{}
	raw, _ := json.Marshal(quote)
	metadata["bepusdt_order"] = string(raw)
	return s.acceptCryptoPayment(ctx, order, quote.Payment.TxHash, order.PayAmount, payment.ProviderBepusdt, metadata)
}

func (s *PaymentService) reconcileBepusdtOrder(ctx context.Context, order *dbent.PaymentOrder) error {
	if bepusdtSnapshotOrder(order) == nil && order.Status == OrderStatusPending {
		_, err := s.resumeBepusdtCreation(ctx, order)
		return err
	}
	prov, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return err
	}
	gateway, ok := prov.(*provider.Bepusdt)
	if !ok {
		return errors.New("invalid gateway provider")
	}
	quote, err := gateway.Query(ctx, order.OutTradeNo)
	if err != nil {
		return err
	}
	if err = s.persistBepusdtOrder(ctx, order, quote); err != nil {
		return err
	}
	if quote.Status == "PAID" {
		return s.acceptBepusdtQuote(ctx, order, quote)
	}
	return nil
}

func (s *PaymentService) cancelBepusdtOrder(ctx context.Context, order *dbent.PaymentOrder, finalStatus, operator string) (string, error) {
	prov, err := s.getOrderProvider(ctx, order)
	if err != nil {
		return "", err
	}
	gateway, ok := prov.(*provider.Bepusdt)
	if !ok {
		return "", errors.New("invalid gateway provider")
	}
	quote, err := gateway.Query(ctx, order.OutTradeNo)
	if err != nil {
		return "", err
	}
	if quote.Status == "PAID" {
		if err = s.acceptBepusdtQuote(ctx, order, quote); err != nil {
			return "", err
		}
		return checkPaidResultAlreadyPaid, nil
	}
	if quote.Status == "CONFIRMING" || quote.Status == "REVIEW" {
		return "", infraerrors.Conflict("PAYMENT_REVIEW", "payment is confirming or under review")
	}
	if err = gateway.CancelPayment(ctx, order.OutTradeNo); err != nil {
		return "", err
	}
	_, err = s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(order.ID), paymentorder.StatusEQ(OrderStatusPending)).SetStatus(finalStatus).Save(ctx)
	if err != nil {
		return "", err
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_CANCELLED", operator, nil)
	return checkPaidResultCancelled, nil
}
