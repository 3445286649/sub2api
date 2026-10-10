package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/cryptopaymentreceipt"
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
			var recovered string
			if raw, exists := nested["recovered_reservation_evidence"]; exists {
				recovered, err = s.reviewedLegacyBSCReservation(ctx, order, raw)
				if err != nil {
					return nil, fmt.Errorf("legacy order %d recovery evidence: %w", order.ID, err)
				}
			}
			if intent == "" && recovered == "" && order.PaidAt == nil && order.Status == OrderStatusFailed {
				continue
			}
			if intent == "" && recovered == "" {
				return nil, fmt.Errorf("legacy order %d is unresolved; finish or reject its payment creation before gateway BSC enablement", order.ID)
			}
			amount, valid := provider.USDTBSCIntentTokenAmount(intent)
			parts := strings.Split(intent, "|")
			if !valid || len(parts) < 5 {
				if recovered != "" {
					distinct[recovered] = struct{}{}
					continue
				}
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
			row := "bsc_mainnet|0x55d398326f99059ff775485246999027b3197955|" + address + "|" + atomic
			if recovered != "" && recovered != row {
				return nil, fmt.Errorf("legacy order %d recovery evidence conflicts with its original payment intent", order.ID)
			}
			distinct[row] = struct{}{}
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

type legacyBSCReservationEvidence struct {
	SchemaVersion  int    `json:"schema_version"`
	Source         string `json:"source"`
	ChainID        string `json:"chain_id"`
	TokenContract  string `json:"token_contract"`
	ReceiveAddress string `json:"receive_address"`
	AmountAtomic   string `json:"amount_atomic"`
	TxHash         string `json:"tx_hash"`
	EventIndex     *int64 `json:"event_index"`
	BlockNumber    int64  `json:"block_number"`
	BlockHash      string `json:"block_hash"`
	BlockTime      string `json:"block_time"`
	ReviewedAt     string `json:"reviewed_at"`
	EvidenceDigest string `json:"evidence_digest"`
}

// Reviewed observations restore only a permanent reservation, never an original
// quote or paid state. The existing durable receipt supplies ownership binding.
func (s *PaymentConfigService) reviewedLegacyBSCReservation(ctx context.Context, order *dbent.PaymentOrder, raw any) (string, error) {
	if order.PaidAt == nil {
		return "", errors.New("reservation recovery requires an already paid order")
	}
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) > 4096 {
		return "", errors.New("invalid reservation recovery evidence")
	}
	var evidence legacyBSCReservationEvidence
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&evidence) != nil || evidence.SchemaVersion != 1 || evidence.Source != "chain_receipt_review" || evidence.ChainID != "bsc_mainnet" || evidence.EventIndex == nil || *evidence.EventIndex < 0 || evidence.BlockNumber <= 0 {
		return "", errors.New("invalid reservation recovery identity or event")
	}
	contract := strings.ToLower(strings.TrimSpace(evidence.TokenContract))
	address := strings.ToLower(strings.TrimSpace(evidence.ReceiveAddress))
	hash := strings.ToLower(strings.TrimSpace(evidence.TxHash))
	if contract != "0x55d398326f99059ff775485246999027b3197955" || !bepusdtLegacyAddressValid(address) || order.QrCode == nil || !strings.EqualFold(strings.TrimSpace(*order.QrCode), address) || !cryptoTxHashPattern.MatchString(hash) || !cryptoTxHashPattern.MatchString(evidence.BlockHash) {
		return "", errors.New("reservation recovery address, contract or hash mismatch")
	}
	amount, ok := new(big.Int).SetString(evidence.AmountAtomic, 10)
	if !ok || amount.Sign() <= 0 || len(evidence.AmountAtomic) > 78 || amount.String() != evidence.AmountAtomic {
		return "", errors.New("invalid reservation recovery atomic amount")
	}
	nested, _ := order.ProviderSnapshot[payment.TypeUSDTBSC].(map[string]any)
	if tokenAmount := psSnapshotStringValue(nested["token_amount"]); tokenAmount != "" {
		atomic, parseErr := provider.BepusdtAmountAtomic(tokenAmount, 18)
		if parseErr != nil || atomic != evidence.AmountAtomic {
			return "", errors.New("reservation recovery amount conflicts with paid order snapshot")
		}
	}
	orderHash := strings.ToLower(strings.TrimSpace(order.PaymentTradeNo))
	snapshotHash := strings.ToLower(strings.TrimSpace(psSnapshotStringValue(nested["tx_hash"])))
	if (cryptoTxHashPattern.MatchString(orderHash) && orderHash != hash) || (snapshotHash != "" && snapshotHash != hash) || (!cryptoTxHashPattern.MatchString(orderHash) && snapshotHash != hash) {
		return "", errors.New("reservation recovery transaction conflicts with paid order")
	}
	blockTime, blockErr := time.Parse(time.RFC3339, evidence.BlockTime)
	reviewedAt, reviewErr := time.Parse(time.RFC3339, evidence.ReviewedAt)
	if blockErr != nil || reviewErr != nil || blockTime.Before(order.CreatedAt) || !blockTime.Before(order.ExpiresAt) || blockTime.After(*order.PaidAt) || reviewedAt.Before(*order.PaidAt) || reviewedAt.After(time.Now().Add(5*time.Minute)) {
		return "", errors.New("reservation recovery times do not match paid order")
	}
	// This digest records provenance, not authentication of the supplied evidence.
	if digest, digestErr := hex.DecodeString(evidence.EvidenceDigest); digestErr != nil || len(digest) != sha256.Size {
		return "", errors.New("invalid reservation recovery provenance digest")
	}
	receipt, err := s.entClient.CryptoPaymentReceipt.Query().Where(cryptopaymentreceipt.ChainIDEQ(evidence.ChainID), cryptopaymentreceipt.TxHashEQ(hash)).Only(ctx)
	if err != nil {
		return "", fmt.Errorf("reservation recovery requires an existing receipt: %w", err)
	}
	if receipt.OrderID != order.ID || !strings.EqualFold(receipt.TokenContract, contract) || receipt.AmountAtomic != evidence.AmountAtomic || receipt.EventIndex == nil || *receipt.EventIndex != *evidence.EventIndex || (receipt.Source != "legacy_usdt_bsc" && receipt.Source != payment.TypeUSDTBSC) {
		return "", errors.New("reservation recovery conflicts with durable receipt")
	}
	return evidence.ChainID + "|" + contract + "|" + address + "|" + evidence.AmountAtomic, nil
}

func bepusdtLegacyAddressValid(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	_, err := hex.DecodeString(address[2:])
	return err == nil
}
