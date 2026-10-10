//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func recoveredLegacyReservationFixture(t *testing.T) (*PaymentConfigService, *dbent.PaymentOrder, *dbent.CryptoPaymentReceipt, map[string]any) {
	t.Helper()
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	user, err := client.User.Create().SetEmail(uuid.NewString() + "@example.com").SetUsername("reservation-review").SetPasswordHash("h").Save(ctx)
	require.NoError(t, err)
	created := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	expires := created.Add(time.Hour)
	paid := expires.Add(time.Minute)
	address := "0x1111111111111111111111111111111111111111"
	hash := "0x" + strings.Repeat("b", 64)
	contract := "0x55d398326f99059ff775485246999027b3197955"
	atomic := "1000123000000000000"
	evidence := map[string]any{"schema_version": 1, "source": "chain_receipt_review", "chain_id": "bsc_mainnet", "token_contract": contract, "receive_address": address, "amount_atomic": atomic, "tx_hash": hash, "event_index": 7, "block_number": 100, "block_hash": "0x" + strings.Repeat("c", 64), "block_time": created.Add(time.Minute).Format(time.RFC3339), "reviewed_at": time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), "evidence_digest": strings.Repeat("d", 64)}
	snapshot := map[string]any{"usdt_bsc": map[string]any{"recovered_reservation_evidence": evidence}}
	order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).SetAmount(7).SetPayAmount(7).SetRechargeCode("PAY-" + uuid.NewString()).SetOutTradeNo("sub2_" + uuid.NewString()).SetPaymentType(payment.TypeUSDTBSC).SetProviderKey(payment.TypeUSDTBSC).SetPaymentTradeNo(hash).SetQrCode(address).SetStatus(OrderStatusCompleted).SetCreatedAt(created).SetExpiresAt(expires).SetPaidAt(paid).SetCompletedAt(paid).SetProviderSnapshot(snapshot).SetClientIP("127.0.0.1").SetSrcHost("example.com").Save(ctx)
	require.NoError(t, err)
	receipt, err := client.CryptoPaymentReceipt.Create().SetChainID("bsc_mainnet").SetTxHash(hash).SetOrderID(order.ID).SetTokenContract(contract).SetAmountAtomic(atomic).SetEventIndex(7).SetSource("legacy_usdt_bsc").Save(ctx)
	require.NoError(t, err)
	return NewPaymentConfigService(client, nil, nil), order, receipt, evidence
}

func TestLegacyBSCReservationExportAcceptsReviewedReceiptOnly(t *testing.T) {
	svc, order, receipt, _ := recoveredLegacyReservationFixture(t)
	rows, digest, err := svc.LegacyBSCReservationExport(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"bsc_mainnet|0x55d398326f99059ff775485246999027b3197955|0x1111111111111111111111111111111111111111|1000123000000000000"}, rows)
	require.Equal(t, legacyBSCReservationDigest(rows), digest)
	fresh, err := svc.entClient.PaymentOrder.Get(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, order.PaymentTradeNo, fresh.PaymentTradeNo)
	require.Equal(t, OrderStatusCompleted, fresh.Status)
	require.Equal(t, order.PaidAt, fresh.PaidAt)
	require.NotContains(t, fresh.ProviderSnapshot["usdt_bsc"], "intent_trade_no")
	freshReceipt, err := svc.entClient.CryptoPaymentReceipt.Get(context.Background(), receipt.ID)
	require.NoError(t, err)
	require.Equal(t, receipt.AmountAtomic, freshReceipt.AmountAtomic)
}

func TestLegacyBSCReservationExportRejectsUntrustedRecovery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(context.Context, *PaymentConfigService, *dbent.PaymentOrder, *dbent.CryptoPaymentReceipt, map[string]any)
	}{
		{"unpaid", func(ctx context.Context, s *PaymentConfigService, o *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.PaymentOrder.UpdateOneID(o.ID).ClearPaidAt().SetStatus(OrderStatusPending).Save(ctx)
			require.NoError(t, err)
		}},
		{"missing_receipt", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			require.NoError(t, s.entClient.CryptoPaymentReceipt.DeleteOneID(r.ID).Exec(ctx))
		}},
		{"receipt_owner_conflict", func(ctx context.Context, s *PaymentConfigService, o *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).SetOrderID(o.ID + 1).Save(ctx)
			require.NoError(t, err)
		}},
		{"receipt_amount_conflict", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).SetAmountAtomic("2000123000000000000").Save(ctx)
			require.NoError(t, err)
		}},
		{"receipt_contract_conflict", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).SetTokenContract("0x2222222222222222222222222222222222222222").Save(ctx)
			require.NoError(t, err)
		}},
		{"receipt_index_conflict", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).SetEventIndex(8).Save(ctx)
			require.NoError(t, err)
		}},
		{"receipt_missing_index", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).ClearEventIndex().Save(ctx)
			require.NoError(t, err)
		}},
		{"receipt_source_conflict", func(ctx context.Context, s *PaymentConfigService, _ *dbent.PaymentOrder, r *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.CryptoPaymentReceipt.UpdateOneID(r.ID).SetSource(payment.ProviderBepusdt).Save(ctx)
			require.NoError(t, err)
		}},
		{"hash_conflict", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["tx_hash"] = "0x" + strings.Repeat("e", 64)
		}},
		{"address_conflict", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["receive_address"] = "0x2222222222222222222222222222222222222222"
		}},
		{"contract_conflict", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["token_contract"] = "0x2222222222222222222222222222222222222222"
		}},
		{"chain_conflict", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["chain_id"] = "tron_mainnet"
		}},
		{"invalid_source", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["source"] = "config_override"
		}},
		{"invalid_version", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["schema_version"] = 2
		}},
		{"fractional_index", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["event_index"] = 7.5
		}},
		{"before_creation", func(_ context.Context, _ *PaymentConfigService, o *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["block_time"] = o.CreatedAt.Add(-time.Second).Format(time.RFC3339)
		}},
		{"at_expiry", func(_ context.Context, _ *PaymentConfigService, o *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["block_time"] = o.ExpiresAt.Format(time.RFC3339)
		}},
		{"chain_after_settlement", func(ctx context.Context, s *PaymentConfigService, o *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, _ map[string]any) {
			_, err := s.entClient.PaymentOrder.UpdateOneID(o.ID).SetPaidAt(o.CreatedAt.Add(30 * time.Second)).Save(ctx)
			require.NoError(t, err)
		}},
		{"reviewed_before_payment", func(_ context.Context, _ *PaymentConfigService, o *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["reviewed_at"] = o.PaidAt.Add(-time.Second).Format(time.RFC3339)
		}},
		{"invalid_atomic", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["amount_atomic"] = "1e18"
		}},
		{"invalid_digest", func(_ context.Context, _ *PaymentConfigService, _ *dbent.PaymentOrder, _ *dbent.CryptoPaymentReceipt, e map[string]any) {
			e["evidence_digest"] = "not-a-digest"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, order, receipt, evidence := recoveredLegacyReservationFixture(t)
			tc.mutate(ctx, svc, order, receipt, evidence)
			_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(map[string]any{"usdt_bsc": map[string]any{"recovered_reservation_evidence": evidence}}).Save(ctx)
			require.NoError(t, err)
			_, _, err = svc.LegacyBSCReservationExport(ctx)
			require.Error(t, err)
		})
	}
}

func TestLegacyBSCReservationExportRejectsConflictingOriginalIntent(t *testing.T) {
	ctx := context.Background()
	svc, order, _, evidence := recoveredLegacyReservationFixture(t)
	for _, amount := range []string{"1.000123", "2.000123"} {
		intent := "usdt_bsc|legacy|7.00|" + amount + "|0x1111111111111111111111111111111111111111"
		_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(map[string]any{"usdt_bsc": map[string]any{"intent_trade_no": intent, "recovered_reservation_evidence": evidence}}).Save(ctx)
		require.NoError(t, err)
		_, _, err = svc.LegacyBSCReservationExport(ctx)
		if amount == "1.000123" {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, "conflicts")
		}
	}
	_, err := svc.entClient.PaymentOrder.UpdateOneID(order.ID).SetProviderSnapshot(map[string]any{"usdt_bsc": map[string]any{"token_amount": "2.000123", "recovered_reservation_evidence": evidence}}).Save(ctx)
	require.NoError(t, err)
	_, _, err = svc.LegacyBSCReservationExport(ctx)
	require.ErrorContains(t, err, "amount conflicts")
}
