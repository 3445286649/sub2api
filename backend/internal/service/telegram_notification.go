package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyTelegramPaymentNotifications = "telegram_payment_notifications"
	TelegramOutcomeRechargeSuccess         = "recharge_success"
	TelegramOutcomePaymentReview           = "payment_review"
)

var (
	telegramBotTokenPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}:[A-Za-z0-9_-]{20,100}$`)
	telegramGroupIDPattern  = regexp.MustCompile(`^-[1-9][0-9]{0,18}$`)
)

type TelegramSettings struct {
	Enabled             bool   `json:"enabled"`
	BotTokenConfigured  bool   `json:"bot_token_configured"`
	ChatID              string `json:"chat_id"`
	VerifyEasyPayOrders bool   `json:"verify_easypay_orders"`
}

type UpdateTelegramSettingsRequest struct {
	Enabled             bool   `json:"enabled"`
	BotToken            string `json:"bot_token"`
	ChatID              string `json:"chat_id"`
	ClearBotToken       bool   `json:"clear_bot_token"`
	VerifyEasyPayOrders bool   `json:"verify_easypay_orders"`
}

type telegramStoredSettings struct {
	Enabled             bool   `json:"enabled"`
	EncryptedBotToken   string `json:"encrypted_bot_token,omitempty"`
	ChatID              string `json:"chat_id"`
	VerifyEasyPayOrders bool   `json:"verify_easypay_orders"`
}

type TelegramPaymentEvent struct {
	EventKey       string    `json:"event_key"`
	OrderID        int64     `json:"order_id"`
	UserID         int64     `json:"user_id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	OccurredAt     time.Time `json:"occurred_at"`
	PayAmount      float64   `json:"pay_amount"`
	Currency       string    `json:"currency"`
	CreditedAmount float64   `json:"credited_amount"`
	BonusAmount    float64   `json:"bonus_amount"`
	PaymentType    string    `json:"payment_type"`
	OutTradeNo     string    `json:"out_trade_no"`
	Outcome        string    `json:"outcome"`
	Reason         string    `json:"reason,omitempty"`
	Status         string    `json:"status"`
	ReceiptAmount  string    `json:"receipt_amount,omitempty"`
	CryptoCurrency string    `json:"crypto_currency,omitempty"`
}

// TelegramOutboxExecutor accepts both Ent and database/sql transactions.
type TelegramOutboxExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type telegramHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type TelegramNotificationService struct {
	settingRepo  SettingRepository
	encryptor    SecretEncryptor
	db           *sql.DB
	httpClient   telegramHTTPClient
	settingsMu   sync.Mutex
	workerMu     sync.Mutex
	workerCancel context.CancelFunc
	workerDone   chan struct{}
}

func NewTelegramNotificationService(settingRepo SettingRepository, encryptor SecretEncryptor, db *sql.DB) *TelegramNotificationService {
	return &TelegramNotificationService{
		settingRepo: settingRepo,
		encryptor:   encryptor,
		db:          db,
		httpClient: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *TelegramNotificationService) loadSettings(ctx context.Context) (*telegramStoredSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("telegram settings are unavailable")
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyTelegramPaymentNotifications)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && raw == "") {
		return &telegramStoredSettings{}, nil
	}
	if err != nil {
		return nil, errors.New("unable to read telegram settings")
	}
	var cfg telegramStoredSettings
	if json.Unmarshal([]byte(raw), &cfg) != nil {
		return nil, errors.New("stored telegram settings are invalid")
	}
	return &cfg, nil
}

func telegramPublicSettings(cfg *telegramStoredSettings) *TelegramSettings {
	return &TelegramSettings{Enabled: cfg.Enabled, BotTokenConfigured: cfg.EncryptedBotToken != "", ChatID: cfg.ChatID, VerifyEasyPayOrders: cfg.VerifyEasyPayOrders}
}

func (s *TelegramNotificationService) GetSettings(ctx context.Context) (*TelegramSettings, error) {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	return telegramPublicSettings(cfg), nil
}

func validateTelegramGroupID(chatID string) error {
	if !telegramGroupIDPattern.MatchString(chatID) {
		return infraerrors.BadRequest("TELEGRAM_CHAT_ID_INVALID", "Telegram chat ID must be a negative group ID")
	}
	if id, err := strconv.ParseInt(chatID, 10, 64); err != nil || id >= 0 {
		return infraerrors.BadRequest("TELEGRAM_CHAT_ID_INVALID", "Telegram group ID is invalid")
	}
	return nil
}

func validateTelegramToken(token string) error {
	if !telegramBotTokenPattern.MatchString(token) {
		return infraerrors.BadRequest("TELEGRAM_BOT_TOKEN_INVALID", "Telegram bot token has an invalid format")
	}
	return nil
}

func (s *TelegramNotificationService) UpdateSettings(ctx context.Context, req UpdateTelegramSettingsRequest) (*TelegramSettings, error) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	token, chatID := strings.TrimSpace(req.BotToken), strings.TrimSpace(req.ChatID)
	if req.ClearBotToken && token != "" {
		return nil, infraerrors.BadRequest("TELEGRAM_BOT_TOKEN_INVALID", "Cannot clear and replace the Telegram bot token together")
	}
	if chatID != "" {
		if err := validateTelegramGroupID(chatID); err != nil {
			return nil, err
		}
	}
	if req.ClearBotToken {
		cfg.EncryptedBotToken = ""
	}
	if token != "" {
		if err := validateTelegramToken(token); err != nil {
			return nil, err
		}
		if s.encryptor == nil {
			return nil, errors.New("telegram token encryption is unavailable")
		}
		ciphertext, err := s.encryptor.Encrypt(token)
		if err != nil {
			return nil, errors.New("unable to encrypt telegram bot token")
		}
		cfg.EncryptedBotToken = ciphertext
	}
	// Empty fields mean "leave the existing value unchanged". This lets the
	// admin toggle notifications without having to resubmit the token/chat ID.
	cfg.Enabled = req.Enabled
	cfg.VerifyEasyPayOrders = req.VerifyEasyPayOrders
	if chatID != "" {
		cfg.ChatID = chatID
	}
	if cfg.Enabled && (cfg.EncryptedBotToken == "" || cfg.ChatID == "") {
		return nil, infraerrors.BadRequest("TELEGRAM_NOT_CONFIGURED", "Telegram bot token and group ID are required before enabling notifications")
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.New("unable to encode telegram settings")
	}
	if err := s.settingRepo.Set(ctx, SettingKeyTelegramPaymentNotifications, string(raw)); err != nil {
		return nil, errors.New("unable to save telegram settings")
	}
	return telegramPublicSettings(cfg), nil
}

func (s *TelegramNotificationService) botToken(cfg *telegramStoredSettings) (string, error) {
	if s.encryptor == nil || cfg.EncryptedBotToken == "" {
		return "", infraerrors.BadRequest("TELEGRAM_NOT_CONFIGURED", "Telegram bot token is not configured")
	}
	token, err := s.encryptor.Decrypt(cfg.EncryptedBotToken)
	if err != nil {
		return "", errors.New("unable to decrypt telegram bot token")
	}
	if err := validateTelegramToken(token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *TelegramNotificationService) Test(ctx context.Context, req UpdateTelegramSettingsRequest) error {
	cfg, err := s.loadSettings(ctx)
	if err != nil {
		return err
	}
	token, chatID := strings.TrimSpace(req.BotToken), strings.TrimSpace(req.ChatID)
	if req.ClearBotToken {
		return infraerrors.BadRequest("TELEGRAM_NOT_CONFIGURED", "Cannot send a test message while clearing the bot token")
	}
	if token == "" {
		token, err = s.botToken(cfg)
		if err != nil {
			return err
		}
	} else if err := validateTelegramToken(token); err != nil {
		return err
	}
	if chatID == "" {
		chatID = cfg.ChatID
	}
	if err := validateTelegramGroupID(chatID); err != nil {
		return err
	}
	return s.sendMessage(ctx, token, chatID, "SubAPI Telegram 通知测试\n状态：配置连接正常")
}

type telegramSendError struct {
	code       int
	permanent  bool
	retryAfter time.Duration
}

func (e *telegramSendError) Error() string {
	if e.code != 0 {
		return fmt.Sprintf("telegram delivery failed (code %d)", e.code)
	}
	return "telegram delivery failed"
}

func (s *TelegramNotificationService) sendMessage(ctx context.Context, token, chatID, text string) error {
	if err := validateTelegramToken(token); err != nil {
		return err
	}
	if err := validateTelegramGroupID(chatID); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true})
	if err != nil {
		return &telegramSendError{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return &telegramSendError{}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		// HTTP errors contain the URL and therefore the bot token; never expose them.
		return &telegramSendError{}
	}
	defer resp.Body.Close()
	var reply struct {
		OK         bool `json:"ok"`
		ErrorCode  int  `json:"error_code"`
		Parameters struct {
			RetryAfter int64 `json:"retry_after"`
		} `json:"parameters"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 16*1024)).Decode(&reply)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && decodeErr == nil && reply.OK {
		return nil
	}
	code := resp.StatusCode
	if reply.ErrorCode != 0 && (resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == reply.ErrorCode) {
		code = reply.ErrorCode
	}
	failed := &telegramSendError{code: code}
	if code == http.StatusTooManyRequests {
		seconds := reply.Parameters.RetryAfter
		if seconds < 1 {
			seconds = 30
		}
		if seconds > 86400 {
			seconds = 86400
		}
		failed.retryAfter = time.Duration(seconds) * time.Second
	}
	return failed
}

func telegramSafeText(value string, maxRunes int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, strings.TrimSpace(value))
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes]) + "..."
	}
	return value
}

func formatTelegramPaymentEvent(event TelegramPaymentEvent) string {
	title, status, amountLabel := "SubAPI 充值成功", "已核验并入账", "实付"
	switch event.Outcome {
	case TelegramOutcomePaymentReview:
		title, status, amountLabel = "SubAPI 支付异常告警", "异常回调已拦截，待复核", "应付"
	}
	when := event.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04:05")
	message := fmt.Sprintf("%s\n用户 ID：%d\n用户名：%s\n邮箱：%s\n时间：%s（北京时间）\n%s：%.2f %s",
		title, event.UserID, telegramSafeText(event.Username, 120), telegramSafeText(event.Email, 254), when,
		amountLabel, event.PayAmount, telegramSafeText(event.Currency, 10))
	if event.Outcome == TelegramOutcomeRechargeSuccess {
		message += fmt.Sprintf("\n入账：%.2f USD\n赠金：%.2f USD", event.CreditedAmount, event.BonusAmount)
	}
	message += fmt.Sprintf("\n支付方式：%s\n订单号：%s\n状态：%s", telegramSafeText(event.PaymentType, 60), telegramSafeText(event.OutTradeNo, 160), status)
	if event.Reason != "" {
		message += "\n核验结论：" + telegramSafeText(event.Reason, 500)
	}
	if event.Status != "" {
		stateLabel := "订单状态"
		if event.Outcome != TelegramOutcomeRechargeSuccess {
			stateLabel = "原订单状态"
		}
		message += "\n" + stateLabel + "：" + telegramSafeText(event.Status, 30)
	}
	if event.CryptoCurrency != "" && event.ReceiptAmount != "" {
		message += "\n链上实收：" + telegramSafeText(event.ReceiptAmount, 80) + " " + telegramSafeText(event.CryptoCurrency, 10)
	}
	return message
}

func validateTelegramPaymentEvent(event TelegramPaymentEvent) error {
	if len(event.EventKey) == 0 || len(event.EventKey) > 200 || event.OrderID <= 0 || event.UserID <= 0 || event.OccurredAt.IsZero() {
		return errors.New("invalid telegram payment event identity")
	}
	switch event.Outcome {
	case TelegramOutcomeRechargeSuccess, TelegramOutcomePaymentReview:
	default:
		return errors.New("invalid telegram payment event outcome")
	}
	for _, amount := range []float64{event.PayAmount, event.CreditedAmount, event.BonusAmount} {
		if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return errors.New("invalid telegram payment event amount")
		}
	}
	if event.ReceiptAmount != "" {
		amount, err := strconv.ParseFloat(event.ReceiptAmount, 64)
		if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) || len(event.ReceiptAmount) > 80 {
			return errors.New("invalid telegram payment crypto receipt amount")
		}
	}
	return nil
}
