package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// The gateway address pool is not shared with SubAPI, so a BSC gateway cannot
// coexist with a legacy BSC allocator. TRON-only gateways remain independent.
func (s *PaymentConfigService) validateCryptoEnablement(ctx context.Context, id int64, key string, cfg map[string]string) error {
	if !isCryptoProviderKey(key) {
		return nil
	}
	// Serialize different provider row updates as well as the same-instance lock.
	if s.entClient.Driver().Dialect() == dialect.Postgres {
		if _, err := s.entClient.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", int64(738294016)); err != nil {
			return err
		}
	}
	bscGateway := key == payment.ProviderBepusdt && slices.Contains(provider.BepusdtNetworks(cfg), "bsc_mainnet")
	if key == payment.TypeUSDTBSC {
		var cursor int64
		for {
			orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.IDGT(cursor), paymentorder.ProviderKeyEQ(payment.ProviderBepusdt)).Order(dbent.Asc(paymentorder.FieldID)).Limit(200).All(ctx)
			if err != nil {
				return err
			}
			for _, order := range orders {
				cursor = order.ID
				snapshot := bepusdtSnapshot(order)
				if psSnapshotStringValue(snapshot["network"]) == "bsc_mainnet" && bepusdtSnapshotOrder(order) == nil && psSnapshotStringValue(snapshot["create_stage"]) != "rejected" {
					return infraerrors.Conflict("CRYPTO_UNKNOWN_INTENT", "a gateway BSC payment intent is unresolved; restore its immutable quote before enabling legacy BSC")
				}
			}
			if len(orders) < 200 {
				break
			}
		}
	}
	instances, err := s.entClient.PaymentProviderInstance.Query().Where(paymentproviderinstance.IDNEQ(id), paymentproviderinstance.EnabledEQ(true), paymentproviderinstance.ProviderKeyIn(payment.TypeUSDTBSC, payment.ProviderBepusdt)).All(ctx)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		otherConfig, err := s.decryptConfig(inst.Config)
		if err != nil {
			return err
		}
		if (bscGateway && inst.ProviderKey == payment.TypeUSDTBSC) || (key == payment.TypeUSDTBSC && inst.ProviderKey == payment.ProviderBepusdt && slices.Contains(provider.BepusdtNetworks(otherConfig), "bsc_mainnet")) {
			return infraerrors.Conflict("CRYPTO_CHANNEL_CONFLICT", "legacy BSC and a BSC gateway cannot accept new orders simultaneously; disable the other channel first")
		}
	}
	if bscGateway {
		backfill := &PaymentService{entClient: s.entClient}
		for {
			if _, err := backfill.BackfillLegacyCryptoReceipts(ctx); err != nil {
				return infraerrors.Conflict("CRYPTO_HISTORY_REVIEW_REQUIRED", err.Error())
			}
			if backfill.legacyCryptoBackfillCursor == 0 {
				break
			}
		}
		rows, err := s.legacyBSCReservationRows(ctx)
		if err != nil {
			return infraerrors.Conflict("CRYPTO_HISTORY_IMPORT_REQUIRED", err.Error())
		}
		gateway, err := provider.NewBepusdt(strconv.FormatInt(id, 10), cfg)
		if err != nil {
			return err
		}
		health, err := gateway.Health(ctx)
		if err != nil {
			return infraerrors.Conflict("CRYPTO_HISTORY_IMPORT_REQUIRED", "unable to verify gateway legacy reservation import")
		}
		proof, ok := health["legacy_reservations"].(map[string]any)
		if !ok || psSnapshotIntValue(proof["count"]) != len(rows) || psSnapshotStringValue(proof["digest"]) != legacyBSCReservationDigest(rows) {
			return infraerrors.Conflict("CRYPTO_HISTORY_IMPORT_REQUIRED", "legacy BSC history must be permanently imported into the gateway before enabling BSC; TRON can be enabled independently")
		}
	}
	return nil
}

func legacyBSCReservationDigest(rows []string) string {
	hash := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(hash[:])
}

// LegacyBSCReservationExport exposes only observation tuples for offline import.
// No key material, user data or orders are sent to the gateway during enablement.
func (s *PaymentConfigService) LegacyBSCReservationExport(ctx context.Context) ([]string, string, error) {
	rows, err := s.legacyBSCReservationRows(ctx)
	if err != nil {
		return nil, "", err
	}
	return rows, legacyBSCReservationDigest(rows), nil
}

func (s *PaymentConfigService) legacyBSCReservationRows(ctx context.Context) ([]string, error) {
	distinct := map[string]struct{}{}
	var cursor int64
	for {
		orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.IDGT(cursor), paymentorder.Or(paymentorder.PaymentTypeEQ(payment.TypeUSDTBSC), paymentorder.ProviderKeyEQ(payment.TypeUSDTBSC))).Order(dbent.Asc(paymentorder.FieldID)).Limit(200).All(ctx)
		if err != nil {
			return nil, err
		}
		for _, order := range orders {
			cursor = order.ID
			nested, _ := order.ProviderSnapshot[payment.TypeUSDTBSC].(map[string]any)
			intent := psSnapshotStringValue(nested["intent_trade_no"])
			if intent == "" {
				intent = order.PaymentTradeNo
			}
			if intent == "" && order.PaidAt == nil && order.Status == OrderStatusFailed {
				continue
			}
			if intent == "" {
				return nil, fmt.Errorf("legacy order %d is unresolved; finish or reject its payment creation before gateway BSC enablement", order.ID)
			}
			amount, valid := provider.USDTBSCIntentTokenAmount(intent)
			parts := strings.Split(intent, "|")
			if !valid || len(parts) < 5 {
				return nil, fmt.Errorf("legacy order %d lacks its immutable exposed payment intent; restore historical evidence before importing", order.ID)
			}
			address := strings.ToLower(strings.TrimSpace(parts[4]))
			if !bepusdtLegacyAddressValid(address) {
				return nil, errors.New("legacy receive address is invalid")
			}
			atomic, err := provider.BepusdtAmountAtomic(amount, 18)
			if err != nil {
				return nil, err
			}
			distinct["bsc_mainnet|0x55d398326f99059ff775485246999027b3197955|"+address+"|"+atomic] = struct{}{}
		}
		if len(orders) < 200 {
			break
		}
	}
	rows := make([]string, 0, len(distinct))
	for row := range distinct {
		rows = append(rows, row)
	}
	sort.Strings(rows)
	return rows, nil
}

func bepusdtLegacyAddressValid(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	_, err := hex.DecodeString(address[2:])
	return err == nil
}
