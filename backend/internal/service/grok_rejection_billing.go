package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

const GrokRejectionBillingMode = "grok_rejection"
const GrokRejectionBillingLabel = "Grok 内容审核拒绝费"

// SourceID describes the provider's billing-ID namespace, not a local channel/key.
type GrokRejectionConfig struct {
	Enabled        bool     `json:"enabled"`
	SourceID       string   `json:"source_id"`
	AllowedOrigins []string `json:"allowed_origins"`
	AmountMode     string   `json:"amount_mode"`
	FixedAmountUSD string   `json:"fixed_amount_usd,omitempty"`
}

var rejectionSourcePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

func ParseGrokRejectionConfig(features map[string]any) (GrokRejectionConfig, error) {
	var cfg GrokRejectionConfig
	raw, ok := features["grok_rejection_billing"]
	if !ok || raw == nil {
		return cfg, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return cfg, err
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("invalid Grok rejection billing config: %w", err)
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	if !rejectionSourcePattern.MatchString(cfg.SourceID) {
		return cfg, errors.New("Grok rejection source_id must be a stable lowercase identifier (1-64 characters)")
	}
	if len(cfg.AllowedOrigins) == 0 || len(cfg.AllowedOrigins) > 16 {
		return cfg, errors.New("Grok rejection billing requires 1-16 trusted HTTPS origins")
	}
	for _, origin := range cfg.AllowedOrigins {
		u, e := url.Parse(origin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return cfg, errors.New("trusted origins must be HTTPS origins without credentials, path or query")
		}
	}
	if cfg.AmountMode == "" {
		cfg.AmountMode = "upstream"
	}
	switch cfg.AmountMode {
	case "upstream":
	case "fixed":
		amount, e := rejectionAmount(cfg.FixedAmountUSD)
		if e != nil {
			return cfg, fmt.Errorf("invalid fixed rejection amount: %w", e)
		}
		cfg.FixedAmountUSD = amount
	default:
		return cfg, errors.New("Grok rejection amount_mode must be upstream or fixed")
	}
	return cfg, nil
}

func (c GrokRejectionConfig) Trusts(target string) bool {
	if !c.Enabled {
		return false
	}
	u, err := url.Parse(target)
	if err != nil || u.User != nil {
		return false
	}
	for _, origin := range c.AllowedOrigins {
		v, err := url.Parse(origin)
		if err == nil && u.Scheme == v.Scheme && strings.EqualFold(u.Host, v.Host) {
			return true
		}
	}
	return false
}

func rejectionAmount(s string) (string, error) {
	if len(s) > 40 {
		return "", errors.New("amount too long")
	}
	n, err := decimal.NewFromString(s)
	if err != nil || !n.IsPositive() || n.GreaterThanOrEqual(decimal.NewFromInt(1_000_000_000_000)) || !n.Equal(n.Round(UsageBillingMonetaryScale)) {
		return "", errors.New("amount must be positive USD with at most 8 decimal places")
	}
	f, _ := n.Float64()
	if !decimal.NewFromFloat(f).Equal(n) {
		return "", errors.New("amount exceeds supported accounting precision")
	}
	return n.String(), nil
}

type GrokRejectionReceipt struct {
	RawChargedAmount string `json:"raw_charged_amount,omitempty"`
	BillingRequestID string `json:"billing_request_id"`
	BillingStatus    string `json:"billing_status"`
	UpstreamAmount   string `json:"charged_amount,omitempty"`
	UserAmount       string `json:"user_amount,omitempty"`
	Currency         string `json:"currency,omitempty"`
	ReviewReason     string `json:"review_reason,omitempty"`
}

func (r GrokRejectionReceipt) Chargeable() bool {
	return r.BillingStatus == "charged" && r.ReviewReason == "" && r.BillingRequestID != "" && r.UpstreamAmount != "" && r.UserAmount != ""
}

// Only protocol-level fields are read; never search generated text/tool arguments.
func ParseGrokRejectionReceipts(body []byte, cfg GrokRejectionConfig) ([]GrokRejectionReceipt, bool, error) {
	var root struct {
		NormalBilling string          `json:"normal_billing"`
		Billing       json.RawMessage `json:"grok_rejection_billing"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false, err
	}
	replaced := root.NormalBilling == "replaced"
	if len(root.Billing) == 0 {
		return nil, replaced, nil
	}
	var billing struct {
		Charges []json.RawMessage `json:"charges"`
	}
	if err := json.Unmarshal(root.Billing, &billing); err != nil {
		return nil, replaced, err
	}
	if len(billing.Charges) > 256 {
		return nil, replaced, errors.New("too many rejection charges")
	}
	result := make([]GrokRejectionReceipt, 0, len(billing.Charges))
	for _, raw := range billing.Charges {
		var wire struct {
			ID       string          `json:"billing_request_id"`
			Status   string          `json:"billing_status"`
			Amount   json.RawMessage `json:"charged_amount"`
			Currency string          `json:"currency"`
		}
		r := GrokRejectionReceipt{}
		if err := json.Unmarshal(raw, &wire); err != nil {
			r.ReviewReason = "invalid_receipt"
			result = append(result, r)
			continue
		}
		r.BillingRequestID = wire.ID
		r.BillingStatus = wire.Status
		r.Currency = wire.Currency
		if strings.ContainsAny(wire.ID+wire.Status+wire.Currency, "\x00") {
			r = GrokRejectionReceipt{ReviewReason: "invalid_receipt_control_character"}
			result = append(result, r)
			continue
		}
		if r.Currency == "" {
			r.Currency = "USD"
		}
		switch {
		case wire.ID == "" || len(wire.ID) > 255 || strings.TrimSpace(wire.ID) != wire.ID:
			r.ReviewReason = "invalid_billing_request_id"
		case r.Currency != "USD":
			r.ReviewReason = "unsupported_currency"
		case wire.Status != "charged":
			r.ReviewReason = "upstream_" + wire.Status
		}
		amount := string(wire.Amount)
		r.RawChargedAmount = amount
		if len(wire.Amount) > 0 && wire.Amount[0] == '"' {
			_ = json.Unmarshal(wire.Amount, &amount)
		}
		n, err := rejectionAmount(amount)
		if err != nil {
			r.ReviewReason = "invalid_charged_amount"
		} else {
			r.UpstreamAmount = n
			r.UserAmount = n
		}
		if cfg.AmountMode == "fixed" && r.UserAmount != "" {
			r.UserAmount = cfg.FixedAmountUSD
		}
		result = append(result, r)
	}
	return result, replaced, nil
}

type GrokRejectionCommand struct {
	AccountType           string               `json:"account_type"`
	SourceID              string               `json:"source_id"`
	LocalRequestID        string               `json:"local_request_id"`
	Receipt               GrokRejectionReceipt `json:"receipt"`
	NormalBillingReplaced bool                 `json:"normal_billing_replaced"`
	Usage                 UsageLog             `json:"usage"`
}

func (c *GrokRejectionCommand) EventKey() string {
	// Price overrides are intentionally excluded: the first received charged
	// receipt freezes the customer price, including when recovery occurs later.
	identity := c.SourceID + "\x00" + c.Receipt.BillingRequestID + "\x00" + c.Receipt.BillingStatus + "\x00" + c.Receipt.UpstreamAmount + fmt.Sprintf("\x00%d", c.Usage.UserID) + "\x00" + c.Receipt.ReviewReason
	if c.Receipt.BillingRequestID == "" {
		identity += "\x00" + c.LocalRequestID
	}
	h := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(h[:])
}
func GrokRejectionUsageRequestID(source, id string) string {
	h := sha256.Sum256([]byte("grok-rejection\x00" + source + "\x00" + id))
	// usage_logs.request_id is VARCHAR(64); the ledger keeps the original ID.
	return hex.EncodeToString(h[:])
}

// Implemented by the existing SQL billing repository, sharing transaction helpers.
type GrokRejectionBillingRepository interface {
	EnqueueGrokRejection(context.Context, *GrokRejectionCommand) error
	SettleGrokRejection(context.Context, string) (*UsageBillingApplyResult, error)
	PendingGrokRejections(context.Context, int) ([]string, error)
	BindGrokRejectionTask(context.Context, string, string, int64, int64) error
	OwnsGrokRejectionTask(context.Context, string, string, int64) (bool, error)
	GrokTaskNormalBillingReplaced(context.Context, string, string, int64) (bool, error)
	ReplaceGrokTaskNormalBilling(context.Context, string, string, int64) error
}
