package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	pointsSettingEnabled       = "points_shop_enabled"
	pointsSettingThreshold     = "points_invite_threshold_amount"
	pointsSettingReward        = "points_invite_reward_points"
	pointsSettingWindowDays    = "points_invite_window_days"
	pointsSettingFreezeHours   = "points_invite_freeze_hours"
	pointsProductTypeBalance   = "balance"
	pointsAwardPending         = "pending"
	pointsAwardAvailable       = "available"
	pointsAwardRevoked         = "revoked"
	pointsDefaultPageSize      = 20
	pointsMaximumPageSize      = 100
	pointsMaximumFeatureLength = 4000
)

type PointsConfig struct {
	Enabled                 bool    `json:"enabled"`
	InviteThresholdAmount   float64 `json:"invite_threshold_amount"`
	InviteRewardPoints      int64   `json:"invite_reward_points"`
	QualificationWindowDays int     `json:"qualification_window_days"`
	FreezeHours             int     `json:"freeze_hours"`
}

type PointsAccountSummary struct {
	AvailablePoints int64 `json:"available_points"`
	FrozenPoints    int64 `json:"frozen_points"`
	DebtPoints      int64 `json:"debt_points"`
	LifetimeEarned  int64 `json:"lifetime_earned"`
	LifetimeSpent   int64 `json:"lifetime_spent"`
}

type PointsProduct struct {
	ID                  int64     `json:"id"`
	ProductType         string    `json:"product_type"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	PointsPrice         int64     `json:"points_price"`
	OriginalPointsPrice *int64    `json:"original_points_price,omitempty"`
	BalanceAmount       float64   `json:"balance_amount"`
	StockTotal          *int64    `json:"stock_total,omitempty"`
	StockRedeemed       int64     `json:"stock_redeemed"`
	PerUserLimit        *int      `json:"per_user_limit,omitempty"`
	Features            string    `json:"features"`
	SortOrder           int       `json:"sort_order"`
	ForSale             bool      `json:"for_sale"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PointsProductInput struct {
	ProductType         string  `json:"product_type"`
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	PointsPrice         int64   `json:"points_price"`
	OriginalPointsPrice *int64  `json:"original_points_price"`
	BalanceAmount       float64 `json:"balance_amount"`
	StockTotal          *int64  `json:"stock_total"`
	PerUserLimit        *int    `json:"per_user_limit"`
	Features            string  `json:"features"`
	SortOrder           int     `json:"sort_order"`
	ForSale             bool    `json:"for_sale"`
}

type PointsShopOrder struct {
	ID            int64     `json:"id"`
	OrderNo       string    `json:"order_no"`
	UserID        int64     `json:"user_id"`
	UserEmail     string    `json:"user_email,omitempty"`
	ProductID     *int64    `json:"product_id,omitempty"`
	ProductName   string    `json:"product_name"`
	ProductType   string    `json:"product_type"`
	PointsPrice   int64     `json:"points_price"`
	BalanceAmount float64   `json:"balance_amount"`
	Status        string    `json:"status"`
	BalanceAfter  float64   `json:"balance_after"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   time.Time `json:"completed_at"`
}

type PointsLedgerEntry struct {
	ID             int64     `json:"id"`
	Action         string    `json:"action"`
	DeltaAvailable int64     `json:"delta_available"`
	DeltaFrozen    int64     `json:"delta_frozen"`
	DeltaDebt      int64     `json:"delta_debt"`
	AvailableAfter int64     `json:"available_after"`
	FrozenAfter    int64     `json:"frozen_after"`
	DebtAfter      int64     `json:"debt_after"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

type PointsPage[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

type PointsShopService struct {
	db                  *sql.DB
	billingCacheService *BillingCacheService
}

const qualifyingRechargeAmountSQL = `
	SELECT COALESCE(SUM(
		CASE
			WHEN jsonb_typeof(provider_snapshot->'base_recharge_amount') = 'number'
				THEN GREATEST(0, (provider_snapshot->>'base_recharge_amount')::numeric)
			ELSE 0
		END * CASE
			WHEN amount > 0 THEN GREATEST(0, 1 - (refund_amount / amount))
			ELSE 0
		END
	), 0)
	FROM payment_orders
	WHERE user_id = $1
	  AND order_type = 'balance'
	  AND status IN ('COMPLETED', 'PARTIALLY_REFUNDED')
	  AND completed_at IS NOT NULL
	  AND completed_at <= $2`

type pointsQueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func queryQualifyingRechargeAmount(ctx context.Context, q pointsQueryRower, inviteeID int64, cutoff time.Time) (float64, error) {
	var amount float64
	err := q.QueryRowContext(ctx, qualifyingRechargeAmountSQL, inviteeID, cutoff).Scan(&amount)
	return amount, err
}

func NewPointsShopService(db *sql.DB, billingCacheService *BillingCacheService) *PointsShopService {
	return &PointsShopService{db: db, billingCacheService: billingCacheService}
}

func (s *PointsShopService) GetConfig(ctx context.Context) (PointsConfig, error) {
	cfg := PointsConfig{Enabled: true, InviteThresholdAmount: 50, InviteRewardPoints: 1, QualificationWindowDays: 30, FreezeHours: 168}
	if s == nil || s.db == nil {
		return cfg, errors.New("points shop database unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings WHERE key IN ($1,$2,$3,$4,$5)`,
		pointsSettingEnabled, pointsSettingThreshold, pointsSettingReward, pointsSettingWindowDays, pointsSettingFreezeHours)
	if err != nil {
		return cfg, fmt.Errorf("load points settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return cfg, err
		}
		switch key {
		case pointsSettingEnabled:
			cfg.Enabled, _ = strconv.ParseBool(strings.TrimSpace(value))
		case pointsSettingThreshold:
			if v, parseErr := strconv.ParseFloat(strings.TrimSpace(value), 64); parseErr == nil {
				cfg.InviteThresholdAmount = v
			}
		case pointsSettingReward:
			if v, parseErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64); parseErr == nil {
				cfg.InviteRewardPoints = v
			}
		case pointsSettingWindowDays:
			if v, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil {
				cfg.QualificationWindowDays = v
			}
		case pointsSettingFreezeHours:
			if v, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil {
				cfg.FreezeHours = v
			}
		}
	}
	return cfg, rows.Err()
}

func (s *PointsShopService) UpdateConfig(ctx context.Context, cfg PointsConfig) (PointsConfig, error) {
	if err := validatePointsConfig(cfg); err != nil {
		return PointsConfig{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PointsConfig{}, err
	}
	defer func() { _ = tx.Rollback() }()
	values := map[string]string{
		pointsSettingEnabled:     strconv.FormatBool(cfg.Enabled),
		pointsSettingThreshold:   strconv.FormatFloat(cfg.InviteThresholdAmount, 'f', -1, 64),
		pointsSettingReward:      strconv.FormatInt(cfg.InviteRewardPoints, 10),
		pointsSettingWindowDays:  strconv.Itoa(cfg.QualificationWindowDays),
		pointsSettingFreezeHours: strconv.Itoa(cfg.FreezeHours),
	}
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`, key, value); err != nil {
			return PointsConfig{}, fmt.Errorf("save %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return PointsConfig{}, err
	}
	return cfg, nil
}

func validatePointsConfig(cfg PointsConfig) error {
	if cfg.InviteThresholdAmount <= 0 || math.IsNaN(cfg.InviteThresholdAmount) || math.IsInf(cfg.InviteThresholdAmount, 0) {
		return infraerrors.BadRequest("POINTS_THRESHOLD_INVALID", "invite threshold must be greater than zero")
	}
	if cfg.InviteRewardPoints <= 0 || cfg.InviteRewardPoints > 1000000 {
		return infraerrors.BadRequest("POINTS_REWARD_INVALID", "invite reward points are out of range")
	}
	if cfg.QualificationWindowDays < 1 || cfg.QualificationWindowDays > 3650 {
		return infraerrors.BadRequest("POINTS_WINDOW_INVALID", "qualification window days are out of range")
	}
	if cfg.FreezeHours < 0 || cfg.FreezeHours > 8760 {
		return infraerrors.BadRequest("POINTS_FREEZE_INVALID", "freeze hours are out of range")
	}
	return nil
}

func (s *PointsShopService) GetAccount(ctx context.Context, userID int64) (PointsAccountSummary, error) {
	if userID <= 0 {
		return PointsAccountSummary{}, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required")
	}
	if err := s.releaseMatured(ctx, userID); err != nil {
		return PointsAccountSummary{}, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO user_points_accounts(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return PointsAccountSummary{}, err
	}
	var result PointsAccountSummary
	err := s.db.QueryRowContext(ctx, `SELECT available_points,frozen_points,debt_points,lifetime_earned,lifetime_spent FROM user_points_accounts WHERE user_id=$1`, userID).
		Scan(&result.AvailablePoints, &result.FrozenPoints, &result.DebtPoints, &result.LifetimeEarned, &result.LifetimeSpent)
	return result, err
}

func (s *PointsShopService) releaseMatured(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_points_accounts(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return err
	}
	account, err := lockPointsAccount(ctx, tx, userID)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.points,a.award_version,a.invitee_user_id,a.threshold_amount,a.qualification_window_days,u.created_at FROM affiliate_point_awards a JOIN users u ON u.id=a.invitee_user_id WHERE a.inviter_user_id=$1 AND a.status='pending' AND a.release_at<=NOW() ORDER BY a.id FOR UPDATE OF a`, userID)
	if err != nil {
		return err
	}
	type maturedAward struct {
		id, points, inviteeID int64
		version, windowDays   int
		threshold             float64
		registeredAt          time.Time
	}
	var awards []maturedAward
	for rows.Next() {
		var item maturedAward
		if err := rows.Scan(&item.id, &item.points, &item.version, &item.inviteeID, &item.threshold, &item.windowDays, &item.registeredAt); err != nil {
			_ = rows.Close()
			return err
		}
		awards = append(awards, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, award := range awards {
		qualifyingAmount, err := queryQualifyingRechargeAmount(ctx, tx, award.inviteeID, award.registeredAt.AddDate(0, 0, award.windowDays))
		if err != nil {
			return err
		}
		if account.FrozenPoints < award.points {
			return fmt.Errorf("points frozen balance invariant failed for user %d", userID)
		}
		account.FrozenPoints -= award.points
		if qualifyingAmount+1e-8 < award.threshold {
			if _, err := tx.ExecContext(ctx, `UPDATE affiliate_point_awards SET status='revoked',qualifying_amount=$2,revoked_at=NOW(),updated_at=NOW() WHERE id=$1 AND status='pending'`, award.id, qualifyingAmount); err != nil {
				return err
			}
			if err := insertPointsLedger(ctx, tx, userID, "earn_revoke", 0, -award.points, 0, account, "invite_award", award.id,
				fmt.Sprintf("award:%d:revoke:v%d", award.id, award.version), "受邀用户退款，冻结积分已撤销"); err != nil {
				return err
			}
			continue
		}
		toDebt := minInt64(account.DebtPoints, award.points)
		account.DebtPoints -= toDebt
		toAvailable := award.points - toDebt
		account.AvailablePoints += toAvailable
		account.LifetimeEarned += award.points
		if _, err := tx.ExecContext(ctx, `UPDATE affiliate_point_awards SET status='available',released_at=NOW(),updated_at=NOW() WHERE id=$1 AND status='pending'`, award.id); err != nil {
			return err
		}
		if err := insertPointsLedger(ctx, tx, userID, "earn_release", toAvailable, -award.points, -toDebt, account, "invite_award", award.id,
			fmt.Sprintf("award:%d:release:v%d", award.id, award.version), "邀请奖励已解冻"); err != nil {
			return err
		}
	}
	if len(awards) > 0 {
		if err := updatePointsAccount(ctx, tx, userID, account); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PointsShopService) RecordPaymentCompletion(ctx context.Context, order *dbent.PaymentOrder) error {
	if s == nil || s.db == nil || order == nil || order.OrderType != payment.OrderTypeBalance {
		return nil
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil || !cfg.Enabled {
		return err
	}
	var inviterID int64
	var registeredAt time.Time
	err = s.db.QueryRowContext(ctx, `SELECT ua.inviter_id,u.created_at FROM user_affiliates ua JOIN users u ON u.id=ua.user_id WHERE ua.user_id=$1 AND ua.inviter_id IS NOT NULL`, order.UserID).
		Scan(&inviterID, &registeredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if inviterID <= 0 || time.Now().After(registeredAt.AddDate(0, 0, cfg.QualificationWindowDays)) {
		return nil
	}
	qualifyingAmount, err := s.qualifyingRechargeAmount(ctx, order.UserID, registeredAt, cfg.QualificationWindowDays)
	if err != nil || qualifyingAmount+1e-8 < cfg.InviteThresholdAmount {
		return err
	}
	return s.createOrRestoreAward(ctx, inviterID, order.UserID, order.ID, qualifyingAmount, cfg)
}

func (s *PointsShopService) qualifyingRechargeAmount(ctx context.Context, inviteeID int64, registeredAt time.Time, windowDays int) (float64, error) {
	return queryQualifyingRechargeAmount(ctx, s.db, inviteeID, registeredAt.AddDate(0, 0, windowDays))
}

func (s *PointsShopService) createOrRestoreAward(ctx context.Context, inviterID, inviteeID, sourceOrderID int64, qualifyingAmount float64, cfg PointsConfig) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_points_accounts(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, inviterID); err != nil {
		return err
	}
	account, err := lockPointsAccount(ctx, tx, inviterID)
	if err != nil {
		return err
	}
	var awardID int64
	var status string
	var version int
	err = tx.QueryRowContext(ctx, `SELECT id,status,award_version FROM affiliate_point_awards WHERE invitee_user_id=$1 FOR UPDATE`, inviteeID).Scan(&awardID, &status, &version)
	if err == nil && status != pointsAwardRevoked {
		return tx.Commit()
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		version = 1
		err = tx.QueryRowContext(ctx, `INSERT INTO affiliate_point_awards(inviter_user_id,invitee_user_id,source_order_id,status,points,threshold_amount,qualifying_amount,qualification_window_days,freeze_hours,award_version,release_at) VALUES($1,$2,$3,'pending',$4,$5,$6,$7,$8,$9,NOW()+($8::integer*INTERVAL '1 hour')) RETURNING id`,
			inviterID, inviteeID, sourceOrderID, cfg.InviteRewardPoints, cfg.InviteThresholdAmount, qualifyingAmount, cfg.QualificationWindowDays, cfg.FreezeHours, version).Scan(&awardID)
	} else {
		version++
		_, err = tx.ExecContext(ctx, `UPDATE affiliate_point_awards SET inviter_user_id=$1,source_order_id=$2,status='pending',points=$3,threshold_amount=$4,qualifying_amount=$5,qualification_window_days=$6,freeze_hours=$7,award_version=$8,release_at=NOW()+($7::integer*INTERVAL '1 hour'),released_at=NULL,revoked_at=NULL,updated_at=NOW() WHERE id=$9`,
			inviterID, sourceOrderID, cfg.InviteRewardPoints, cfg.InviteThresholdAmount, qualifyingAmount, cfg.QualificationWindowDays, cfg.FreezeHours, version, awardID)
	}
	if err != nil {
		return err
	}
	account.FrozenPoints += cfg.InviteRewardPoints
	if err := updatePointsAccount(ctx, tx, inviterID, account); err != nil {
		return err
	}
	if err := insertPointsLedger(ctx, tx, inviterID, "earn_pending", 0, cfg.InviteRewardPoints, 0, account, "invite_award", awardID,
		fmt.Sprintf("award:%d:pending:v%d", awardID, version), "有效邀请奖励冻结中"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if cfg.FreezeHours == 0 {
		return s.releaseMatured(ctx, inviterID)
	}
	return nil
}

func (s *PointsShopService) HandlePaymentRefund(ctx context.Context, inviteeID int64) error {
	if s == nil || s.db == nil || inviteeID <= 0 {
		return nil
	}
	var inviterID int64
	err := s.db.QueryRowContext(ctx, `SELECT inviter_user_id FROM affiliate_point_awards WHERE invitee_user_id=$1 AND status IN ('pending','available')`, inviteeID).Scan(&inviterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	account, err := lockPointsAccount(ctx, tx, inviterID)
	if err != nil {
		return err
	}
	var awardID, points int64
	var status string
	var threshold float64
	var windowDays, version int
	var registeredAt time.Time
	err = tx.QueryRowContext(ctx, `SELECT a.id,a.points,a.status,a.threshold_amount,a.qualification_window_days,a.award_version,u.created_at FROM affiliate_point_awards a JOIN users u ON u.id=a.invitee_user_id WHERE a.invitee_user_id=$1 AND a.inviter_user_id=$2 FOR UPDATE OF a`, inviteeID, inviterID).
		Scan(&awardID, &points, &status, &threshold, &windowDays, &version, &registeredAt)
	if errors.Is(err, sql.ErrNoRows) || status == pointsAwardRevoked {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	amount, err := queryQualifyingRechargeAmount(ctx, tx, inviteeID, registeredAt.AddDate(0, 0, windowDays))
	if err != nil || amount+1e-8 >= threshold {
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	deltaAvailable, deltaFrozen, deltaDebt := int64(0), int64(0), int64(0)
	if status == pointsAwardPending {
		if account.FrozenPoints < points {
			return fmt.Errorf("points frozen balance invariant failed for user %d", inviterID)
		}
		account.FrozenPoints -= points
	} else {
		deduct := minInt64(account.AvailablePoints, points)
		account.AvailablePoints -= deduct
		account.DebtPoints += points - deduct
		deltaAvailable = -deduct
		deltaDebt = points - deduct
	}
	if _, err := tx.ExecContext(ctx, `UPDATE affiliate_point_awards SET status='revoked',revoked_at=NOW(),updated_at=NOW() WHERE id=$1 AND status=$2`, awardID, status); err != nil {
		return err
	}
	if err := updatePointsAccount(ctx, tx, inviterID, account); err != nil {
		return err
	}
	if status == pointsAwardPending {
		deltaFrozen = -points
	}
	if err := insertPointsLedger(ctx, tx, inviterID, "earn_revoke", deltaAvailable, deltaFrozen, deltaDebt, account, "invite_award", awardID,
		fmt.Sprintf("award:%d:revoke:v%d", awardID, version), "受邀用户退款，邀请积分已撤销"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PointsShopService) ListProducts(ctx context.Context, includeOffSale bool) ([]PointsProduct, error) {
	query := `SELECT id,product_type,name,description,points_price,original_points_price,balance_amount,stock_total,stock_redeemed,per_user_limit,features,sort_order,for_sale,created_at,updated_at FROM points_shop_products`
	if !includeOffSale {
		query += ` WHERE for_sale=TRUE AND (stock_total IS NULL OR stock_redeemed<stock_total)`
	}
	query += ` ORDER BY sort_order ASC,id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]PointsProduct, 0)
	for rows.Next() {
		product, err := scanPointsProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (s *PointsShopService) CreateProduct(ctx context.Context, input PointsProductInput) (PointsProduct, error) {
	if err := validatePointsProductInput(input); err != nil {
		return PointsProduct{}, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO points_shop_products(product_type,name,description,points_price,original_points_price,balance_amount,stock_total,per_user_limit,features,sort_order,for_sale) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		normalizePointsProductType(input.ProductType), strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), input.PointsPrice, input.OriginalPointsPrice, input.BalanceAmount, input.StockTotal, input.PerUserLimit, strings.TrimSpace(input.Features), input.SortOrder, input.ForSale).Scan(&id)
	if err != nil {
		return PointsProduct{}, err
	}
	return s.getProduct(ctx, id, false)
}

func (s *PointsShopService) UpdateProduct(ctx context.Context, id int64, input PointsProductInput) (PointsProduct, error) {
	if id <= 0 {
		return PointsProduct{}, infraerrors.BadRequest("POINTS_PRODUCT_ID_INVALID", "invalid product id")
	}
	if err := validatePointsProductInput(input); err != nil {
		return PointsProduct{}, err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE points_shop_products SET product_type=$1,name=$2,description=$3,points_price=$4,original_points_price=$5,balance_amount=$6,stock_total=$7,per_user_limit=$8,features=$9,sort_order=$10,for_sale=$11,updated_at=NOW() WHERE id=$12 AND ($7::bigint IS NULL OR $7::bigint>=stock_redeemed)`,
		normalizePointsProductType(input.ProductType), strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), input.PointsPrice, input.OriginalPointsPrice, input.BalanceAmount, input.StockTotal, input.PerUserLimit, strings.TrimSpace(input.Features), input.SortOrder, input.ForSale, id)
	if err != nil {
		return PointsProduct{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return PointsProduct{}, infraerrors.BadRequest("POINTS_PRODUCT_STOCK_INVALID", "stock total cannot be below redeemed stock")
	}
	return s.getProduct(ctx, id, false)
}

func (s *PointsShopService) DeleteProduct(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM points_shop_products WHERE id=$1 AND stock_redeemed=0`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return infraerrors.Conflict("POINTS_PRODUCT_IN_USE", "product with redemption history can only be taken off sale")
	}
	return nil
}

func (s *PointsShopService) getProduct(ctx context.Context, id int64, forUpdate bool) (PointsProduct, error) {
	query := `SELECT id,product_type,name,description,points_price,original_points_price,balance_amount,stock_total,stock_redeemed,per_user_limit,features,sort_order,for_sale,created_at,updated_at FROM points_shop_products WHERE id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	product, err := scanPointsProduct(s.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return PointsProduct{}, infraerrors.NotFound("POINTS_PRODUCT_NOT_FOUND", "product not found")
	}
	return product, err
}

func validatePointsProductInput(input PointsProductInput) error {
	if normalizePointsProductType(input.ProductType) != pointsProductTypeBalance {
		return infraerrors.BadRequest("POINTS_PRODUCT_TYPE_INVALID", "only balance products are supported")
	}
	if name := strings.TrimSpace(input.Name); name == "" || len(name) > 100 {
		return infraerrors.BadRequest("POINTS_PRODUCT_NAME_INVALID", "product name is required and must not exceed 100 characters")
	}
	if input.PointsPrice <= 0 {
		return infraerrors.BadRequest("POINTS_PRODUCT_PRICE_INVALID", "points price must be greater than zero")
	}
	if input.OriginalPointsPrice != nil && *input.OriginalPointsPrice < input.PointsPrice {
		return infraerrors.BadRequest("POINTS_PRODUCT_ORIGINAL_PRICE_INVALID", "original points price cannot be lower than points price")
	}
	if input.BalanceAmount <= 0 || math.IsNaN(input.BalanceAmount) || math.IsInf(input.BalanceAmount, 0) {
		return infraerrors.BadRequest("POINTS_PRODUCT_BALANCE_INVALID", "balance amount must be greater than zero")
	}
	if input.StockTotal != nil && *input.StockTotal < 0 {
		return infraerrors.BadRequest("POINTS_PRODUCT_STOCK_INVALID", "stock total cannot be negative")
	}
	if input.PerUserLimit != nil && *input.PerUserLimit <= 0 {
		return infraerrors.BadRequest("POINTS_PRODUCT_LIMIT_INVALID", "per-user limit must be greater than zero")
	}
	if len(input.Features) > pointsMaximumFeatureLength {
		return infraerrors.BadRequest("POINTS_PRODUCT_FEATURES_TOO_LONG", "features are too long")
	}
	return nil
}

func normalizePointsProductType(value string) string {
	if strings.TrimSpace(value) == "" {
		return pointsProductTypeBalance
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *PointsShopService) Redeem(ctx context.Context, userID, productID int64, idempotencyKey string) (PointsShopOrder, error) {
	key := strings.TrimSpace(idempotencyKey)
	if userID <= 0 || productID <= 0 || key == "" || len(key) > 64 {
		return PointsShopOrder{}, infraerrors.BadRequest("POINTS_REDEEM_INVALID", "invalid redemption request")
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return PointsShopOrder{}, err
	}
	if !cfg.Enabled {
		return PointsShopOrder{}, infraerrors.Forbidden("POINTS_SHOP_DISABLED", "points shop is disabled")
	}
	if err := s.releaseMatured(ctx, userID); err != nil {
		return PointsShopOrder{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PointsShopOrder{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, found, err := findPointsOrderByIdempotency(ctx, tx, userID, key); err != nil {
		return PointsShopOrder{}, err
	} else if found {
		return existing, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_points_accounts(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return PointsShopOrder{}, err
	}
	account, err := lockPointsAccount(ctx, tx, userID)
	if err != nil {
		return PointsShopOrder{}, err
	}
	// A concurrent request with the same key may have committed while this
	// transaction waited for the per-user account lock.
	if existing, found, err := findPointsOrderByIdempotency(ctx, tx, userID, key); err != nil {
		return PointsShopOrder{}, err
	} else if found {
		return existing, nil
	}
	if account.DebtPoints > 0 {
		return PointsShopOrder{}, infraerrors.Conflict("POINTS_DEBT_OUTSTANDING", "outstanding points debt must be cleared before redemption")
	}
	product, err := scanPointsProduct(tx.QueryRowContext(ctx, `SELECT id,product_type,name,description,points_price,original_points_price,balance_amount,stock_total,stock_redeemed,per_user_limit,features,sort_order,for_sale,created_at,updated_at FROM points_shop_products WHERE id=$1 FOR UPDATE`, productID))
	if errors.Is(err, sql.ErrNoRows) {
		return PointsShopOrder{}, infraerrors.NotFound("POINTS_PRODUCT_NOT_FOUND", "product not found")
	}
	if err != nil {
		return PointsShopOrder{}, err
	}
	if !product.ForSale {
		return PointsShopOrder{}, infraerrors.Conflict("POINTS_PRODUCT_OFF_SALE", "product is not for sale")
	}
	if product.StockTotal != nil && product.StockRedeemed >= *product.StockTotal {
		return PointsShopOrder{}, infraerrors.Conflict("POINTS_PRODUCT_SOLD_OUT", "product is sold out")
	}
	if product.PerUserLimit != nil {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM points_shop_orders WHERE user_id=$1 AND product_id=$2 AND status='completed'`, userID, productID).Scan(&count); err != nil {
			return PointsShopOrder{}, err
		}
		if count >= *product.PerUserLimit {
			return PointsShopOrder{}, infraerrors.Conflict("POINTS_PRODUCT_LIMIT_REACHED", "per-user redemption limit reached")
		}
	}
	if account.AvailablePoints < product.PointsPrice {
		return PointsShopOrder{}, infraerrors.Conflict("POINTS_NOT_ENOUGH", "insufficient points")
	}
	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=NOW() WHERE id=$2 AND status='active' AND deleted_at IS NULL RETURNING balance`, product.BalanceAmount, userID).Scan(&balanceAfter); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PointsShopOrder{}, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
		}
		return PointsShopOrder{}, err
	}
	orderNo := "PS" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	var order PointsShopOrder
	err = tx.QueryRowContext(ctx, `INSERT INTO points_shop_orders(order_no,user_id,product_id,idempotency_key,product_name,product_type,points_price,balance_amount,status,balance_after) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'completed',$9) RETURNING id,order_no,user_id,product_id,product_name,product_type,points_price,balance_amount,status,balance_after,created_at,completed_at`,
		orderNo, userID, product.ID, key, product.Name, product.ProductType, product.PointsPrice, product.BalanceAmount, balanceAfter).
		Scan(&order.ID, &order.OrderNo, &order.UserID, &order.ProductID, &order.ProductName, &order.ProductType, &order.PointsPrice, &order.BalanceAmount, &order.Status, &order.BalanceAfter, &order.CreatedAt, &order.CompletedAt)
	if err != nil {
		return PointsShopOrder{}, err
	}
	account.AvailablePoints -= product.PointsPrice
	account.LifetimeSpent += product.PointsPrice
	if err := updatePointsAccount(ctx, tx, userID, account); err != nil {
		return PointsShopOrder{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE points_shop_products SET stock_redeemed=stock_redeemed+1,updated_at=NOW() WHERE id=$1`, product.ID); err != nil {
		return PointsShopOrder{}, err
	}
	if err := insertPointsLedger(ctx, tx, userID, "redeem", -product.PointsPrice, 0, 0, account, "shop_order", order.ID,
		fmt.Sprintf("shop-order:%d", order.ID), "积分兑换："+product.Name); err != nil {
		return PointsShopOrder{}, err
	}
	if err := tx.Commit(); err != nil {
		return PointsShopOrder{}, err
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, userID)
	}
	return order, nil
}

func (s *PointsShopService) ListUserLedger(ctx context.Context, userID int64, page, pageSize int) (PointsPage[PointsLedgerEntry], error) {
	page, pageSize = normalizePointsPage(page, pageSize)
	if err := s.releaseMatured(ctx, userID); err != nil {
		return PointsPage[PointsLedgerEntry]{}, err
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_points_ledger WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return PointsPage[PointsLedgerEntry]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,action,delta_available,delta_frozen,delta_debt,available_after,frozen_after,debt_after,description,created_at FROM user_points_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return PointsPage[PointsLedgerEntry]{}, err
	}
	defer rows.Close()
	items := make([]PointsLedgerEntry, 0)
	for rows.Next() {
		var item PointsLedgerEntry
		if err := rows.Scan(&item.ID, &item.Action, &item.DeltaAvailable, &item.DeltaFrozen, &item.DeltaDebt, &item.AvailableAfter, &item.FrozenAfter, &item.DebtAfter, &item.Description, &item.CreatedAt); err != nil {
			return PointsPage[PointsLedgerEntry]{}, err
		}
		items = append(items, item)
	}
	return PointsPage[PointsLedgerEntry]{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

func (s *PointsShopService) ListOrders(ctx context.Context, userID int64, admin bool, page, pageSize int) (PointsPage[PointsShopOrder], error) {
	page, pageSize = normalizePointsPage(page, pageSize)
	where, args := "", []any{}
	if !admin {
		where = " WHERE o.user_id=$1"
		args = append(args, userID)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM points_shop_orders o`+where, args...).Scan(&total); err != nil {
		return PointsPage[PointsShopOrder]{}, err
	}
	limitPos := len(args) + 1
	args = append(args, pageSize, (page-1)*pageSize)
	query := fmt.Sprintf(`SELECT o.id,o.order_no,o.user_id,u.email,o.product_id,o.product_name,o.product_type,o.points_price,o.balance_amount,o.status,o.balance_after,o.created_at,o.completed_at FROM points_shop_orders o JOIN users u ON u.id=o.user_id%s ORDER BY o.id DESC LIMIT $%d OFFSET $%d`, where, limitPos, limitPos+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return PointsPage[PointsShopOrder]{}, err
	}
	defer rows.Close()
	items := make([]PointsShopOrder, 0)
	for rows.Next() {
		var item PointsShopOrder
		if err := rows.Scan(&item.ID, &item.OrderNo, &item.UserID, &item.UserEmail, &item.ProductID, &item.ProductName, &item.ProductType, &item.PointsPrice, &item.BalanceAmount, &item.Status, &item.BalanceAfter, &item.CreatedAt, &item.CompletedAt); err != nil {
			return PointsPage[PointsShopOrder]{}, err
		}
		items = append(items, item)
	}
	return PointsPage[PointsShopOrder]{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

type pointsAccountState struct {
	AvailablePoints int64
	FrozenPoints    int64
	DebtPoints      int64
	LifetimeEarned  int64
	LifetimeSpent   int64
}

func lockPointsAccount(ctx context.Context, tx *sql.Tx, userID int64) (pointsAccountState, error) {
	var account pointsAccountState
	err := tx.QueryRowContext(ctx, `SELECT available_points,frozen_points,debt_points,lifetime_earned,lifetime_spent FROM user_points_accounts WHERE user_id=$1 FOR UPDATE`, userID).
		Scan(&account.AvailablePoints, &account.FrozenPoints, &account.DebtPoints, &account.LifetimeEarned, &account.LifetimeSpent)
	return account, err
}

func updatePointsAccount(ctx context.Context, tx *sql.Tx, userID int64, account pointsAccountState) error {
	_, err := tx.ExecContext(ctx, `UPDATE user_points_accounts SET available_points=$1,frozen_points=$2,debt_points=$3,lifetime_earned=$4,lifetime_spent=$5,updated_at=NOW() WHERE user_id=$6`,
		account.AvailablePoints, account.FrozenPoints, account.DebtPoints, account.LifetimeEarned, account.LifetimeSpent, userID)
	return err
}

func insertPointsLedger(ctx context.Context, tx *sql.Tx, userID int64, action string, deltaAvailable, deltaFrozen, deltaDebt int64, account pointsAccountState, sourceType string, sourceID int64, businessKey, description string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO user_points_ledger(user_id,action,delta_available,delta_frozen,delta_debt,available_after,frozen_after,debt_after,source_type,source_id,business_key,description) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(business_key) DO NOTHING`,
		userID, action, deltaAvailable, deltaFrozen, deltaDebt, account.AvailablePoints, account.FrozenPoints, account.DebtPoints, sourceType, sourceID, businessKey, description)
	return err
}

type pointsProductScanner interface{ Scan(...any) error }

func scanPointsProduct(scanner pointsProductScanner) (PointsProduct, error) {
	var product PointsProduct
	err := scanner.Scan(&product.ID, &product.ProductType, &product.Name, &product.Description, &product.PointsPrice, &product.OriginalPointsPrice, &product.BalanceAmount, &product.StockTotal, &product.StockRedeemed, &product.PerUserLimit, &product.Features, &product.SortOrder, &product.ForSale, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func findPointsOrderByIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (PointsShopOrder, bool, error) {
	var order PointsShopOrder
	err := tx.QueryRowContext(ctx, `SELECT id,order_no,user_id,product_id,product_name,product_type,points_price,balance_amount,status,balance_after,created_at,completed_at FROM points_shop_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, key).
		Scan(&order.ID, &order.OrderNo, &order.UserID, &order.ProductID, &order.ProductName, &order.ProductType, &order.PointsPrice, &order.BalanceAmount, &order.Status, &order.BalanceAfter, &order.CreatedAt, &order.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PointsShopOrder{}, false, nil
	}
	return order, err == nil, err
}

func normalizePointsPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = pointsDefaultPageSize
	}
	if pageSize > pointsMaximumPageSize {
		pageSize = pointsMaximumPageSize
	}
	return page, pageSize
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
