package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

type usageRebateRepository struct {
	db *sql.DB
}

func NewUsageRebateRepository(db *sql.DB) service.UsageRebateRepository {
	return &usageRebateRepository{db: db}
}

func (r *usageRebateRepository) EnsureOpenPeriod(ctx context.Context, seed service.UsageRebatePeriodSeed) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO usage_rebate_periods (
    business_date, window_start, window_end, settle_after, timezone, rule_version, status, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,'open',NOW(),NOW())
ON CONFLICT (business_date) DO NOTHING`,
		seed.BusinessDate, seed.WindowStart, seed.WindowEnd, seed.SettleAfter, seed.Timezone, seed.RuleVersion)
	return err
}

func (r *usageRebateRepository) ClaimDuePeriod(ctx context.Context, now, lockUntil time.Time) (*service.UsageRebatePeriod, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
SELECT id, business_date::text, window_start, window_end, settle_after, timezone, rule_version,
       rates, status, total_spend, total_reward, attempt_count, error_message, lock_token,
       locked_until, settled_at, created_at, updated_at
FROM usage_rebate_periods
WHERE settle_after <= $1
  AND (
      status IN ('open','failed')
      OR (status = 'settling' AND (locked_until IS NULL OR locked_until <= $1))
  )
ORDER BY settle_after ASC, id ASC
LIMIT 1
FOR UPDATE SKIP LOCKED`, now)
	period, err := scanUsageRebatePeriod(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	token, err := usageRebateLockToken()
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE usage_rebate_periods
SET status='settling', lock_token=$2, locked_until=$3, attempt_count=attempt_count+1,
    error_message='', updated_at=NOW()
WHERE id=$1`, period.ID, token, lockUntil); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	period.Status = service.UsageRebatePeriodStatusSettling
	period.SettlementToken = token
	period.LockedUntil = &lockUntil
	period.AttemptCount++
	return period, nil
}

func (r *usageRebateRepository) SealClaimedPeriod(ctx context.Context, periodID int64, rates []service.UsageRebateRate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var businessDate string
	var start, end time.Time
	var sealedAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `
SELECT business_date::text, window_start, window_end, sealed_at
FROM usage_rebate_periods
WHERE id=$1
FOR UPDATE`, periodID).Scan(&businessDate, &start, &end, &sealedAt); err != nil {
		return err
	}
	if sealedAt.Valid {
		return tx.Commit()
	}

	candidates, err := queryUsageRebateCandidates(ctx, tx, start, end, 0, 20)
	if err != nil {
		return err
	}
	var totalSpend, totalReward decimal.Decimal
	for index, candidate := range candidates {
		rank := index + 1
		if rank > len(rates) {
			break
		}
		reward, ok := service.CalculateUsageRebate(candidate.SpendAmount, rank)
		if !ok {
			continue
		}
		businessKey := fmt.Sprintf("usage-rebate:%s:%d", businessDate, candidate.UserID)
		if _, err = tx.ExecContext(ctx, `
INSERT INTO usage_rebate_rewards (
    period_id, business_date, user_id, rank, spend_amount, rebate_percent,
    reward_amount, status, business_key, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,'pending',$8,NOW(),NOW())
ON CONFLICT (business_key) DO NOTHING`,
			periodID, businessDate, candidate.UserID, rank, candidate.SpendAmount, rates[rank-1].Percent, reward, businessKey); err != nil {
			return err
		}
		totalSpend = totalSpend.Add(candidate.SpendAmount)
		totalReward = totalReward.Add(reward)
	}
	ratesJSON, err := json.Marshal(rates)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE usage_rebate_periods
SET rates=$2, total_spend=$3, total_reward=$4, sealed_at=NOW(), updated_at=NOW()
WHERE id=$1`, periodID, ratesJSON, totalSpend, totalReward); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *usageRebateRepository) ListPayableRewards(ctx context.Context, periodID int64) ([]service.UsageRebateReward, error) {
	rows, err := r.db.QueryContext(ctx, usageRebateRewardSelect+`
WHERE rr.period_id=$1 AND rr.status IN ('pending','failed')
ORDER BY rr.rank ASC, rr.id ASC`, periodID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsageRebateRewards(rows)
}

func (r *usageRebateRepository) CreditReward(ctx context.Context, rewardID int64) (int64, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	var status string
	var amount decimal.Decimal
	if err = tx.QueryRowContext(ctx, `
SELECT user_id, status, reward_amount
FROM usage_rebate_rewards
WHERE id=$1
FOR UPDATE`, rewardID).Scan(&userID, &status, &amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, tx.Commit()
		}
		return 0, false, err
	}
	if status == service.UsageRebateRewardStatusCredited || status == service.UsageRebateRewardStatusUnknown {
		return userID, false, tx.Commit()
	}
	if status != service.UsageRebateRewardStatusPending && status != service.UsageRebateRewardStatusFailed {
		return userID, false, tx.Commit()
	}
	if !amount.IsPositive() {
		return userID, false, errors.New("usage rebate reward amount must be positive")
	}

	var balanceBefore, balanceAfter decimal.Decimal
	if err = tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&balanceBefore); err != nil {
		return userID, false, err
	}
	if err = tx.QueryRowContext(ctx, `
UPDATE users
SET balance=balance+$2, updated_at=NOW()
WHERE id=$1
RETURNING balance`, userID, amount).Scan(&balanceAfter); err != nil {
		return userID, false, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE usage_rebate_rewards
SET status='credited', balance_before=$2, balance_after=$3, credited_at=NOW(),
    error_message='', updated_at=NOW()
WHERE id=$1 AND status IN ('pending','failed')`, rewardID, balanceBefore, balanceAfter); err != nil {
		return userID, false, err
	}
	if err = tx.Commit(); err != nil {
		return userID, false, errors.Join(service.ErrUsageRebateCommitUnknown, err)
	}
	return userID, true, nil
}

func (r *usageRebateRepository) MarkRewardFailed(ctx context.Context, rewardID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE usage_rebate_rewards
SET status='failed', error_message=$2, updated_at=NOW()
WHERE id=$1 AND status IN ('pending','failed')`, rewardID, reason)
	return err
}

func (r *usageRebateRepository) MarkRewardUnknown(ctx context.Context, rewardID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE usage_rebate_rewards
SET status='unknown', error_message=$2, updated_at=NOW()
WHERE id=$1 AND status IN ('pending','failed')`, rewardID, reason)
	return err
}

func (r *usageRebateRepository) FinalizePeriod(ctx context.Context, periodID int64) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE usage_rebate_periods p
SET status = CASE
        WHEN EXISTS (SELECT 1 FROM usage_rebate_rewards r WHERE r.period_id=p.id AND r.status='unknown') THEN 'unknown'
        WHEN EXISTS (SELECT 1 FROM usage_rebate_rewards r WHERE r.period_id=p.id AND r.status IN ('pending','failed')) THEN 'failed'
        ELSE 'settled'
    END,
    settled_at = CASE
        WHEN NOT EXISTS (SELECT 1 FROM usage_rebate_rewards r WHERE r.period_id=p.id AND r.status <> 'credited') THEN NOW()
        ELSE NULL
    END,
    lock_token=NULL, locked_until=NULL, updated_at=NOW()
WHERE p.id=$1 AND p.status='settling'`, periodID)
	return err
}

func (r *usageRebateRepository) MarkPeriodFailed(ctx context.Context, periodID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE usage_rebate_periods
SET status='failed', error_message=$2, lock_token=NULL, locked_until=NULL, updated_at=NOW()
WHERE id=$1 AND status NOT IN ('settled','unknown')`, periodID, reason)
	return err
}

func (r *usageRebateRepository) GetLeaderboard(ctx context.Context, start, end time.Time, viewerUserID int64, limit int) ([]service.UsageRebateCandidate, error) {
	return queryUsageRebateCandidates(ctx, r.db, start, end, viewerUserID, limit)
}

func (r *usageRebateRepository) GetUserPosition(ctx context.Context, start, end time.Time, userID int64) (service.UsageRebatePosition, error) {
	var position service.UsageRebatePosition
	var rank sql.NullInt64
	var previousSpend, top20Spend sql.NullString
	err := r.db.QueryRowContext(ctx, `
WITH totals AS (
    SELECT ul.user_id,
           COUNT(*)::bigint AS requests,
           COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS tokens,
           COALESCE(SUM(ul.actual_cost), 0)::numeric(20,8) AS spend_amount
    FROM usage_logs ul
    JOIN users u ON u.id=ul.user_id
    WHERE ul.created_at >= $1 AND ul.created_at < $2
      AND ul.billing_type=0 AND ul.actual_cost > 0
    GROUP BY ul.user_id
), ranked AS (
    SELECT totals.*,
           ROW_NUMBER() OVER (
               ORDER BY spend_amount DESC, tokens DESC, user_id ASC
           )::integer AS rank
    FROM totals
), annotated AS (
    SELECT ranked.*,
           LAG(spend_amount) OVER (ORDER BY rank) AS previous_spend
    FROM ranked
), summary AS (
    SELECT COUNT(*)::integer AS participant_count,
           MAX(spend_amount) FILTER (WHERE rank=20) AS top_20_spend
    FROM ranked
)
SELECT target.rank, summary.participant_count,
       COALESCE(target.requests, 0), COALESCE(target.tokens, 0), COALESCE(target.spend_amount, 0),
       target.previous_spend::text, summary.top_20_spend::text
FROM summary
LEFT JOIN annotated target ON target.user_id=$3`, start, end, userID).Scan(
		&rank, &position.ParticipantCount, &position.Requests, &position.Tokens, &position.SpendAmount,
		&previousSpend, &top20Spend,
	)
	if err != nil {
		return service.UsageRebatePosition{}, err
	}
	if !rank.Valid {
		return position, nil
	}

	value := int(rank.Int64)
	position.Rank = &value
	position.Eligible = value >= 1 && value <= usageRebateTopLimit()
	if position.Eligible {
		position.RebatePercent = service.UsageRebateRates()[value-1].Percent
		position.EstimatedReward, _ = service.CalculateUsageRebate(position.SpendAmount, value)
	}
	if value > 1 {
		previousRank := value - 1
		position.PreviousRank = &previousRank
		if previousSpend.Valid {
			gap, parseErr := decimal.NewFromString(previousSpend.String)
			if parseErr != nil {
				return service.UsageRebatePosition{}, parseErr
			}
			gap = gap.Sub(position.SpendAmount)
			if gap.IsNegative() {
				gap = decimal.Zero
			}
			position.GapToPrevious = &gap
		}
	}
	if value > usageRebateTopLimit() && top20Spend.Valid {
		gap, parseErr := decimal.NewFromString(top20Spend.String)
		if parseErr != nil {
			return service.UsageRebatePosition{}, parseErr
		}
		gap = gap.Sub(position.SpendAmount)
		if gap.IsNegative() {
			gap = decimal.Zero
		}
		position.GapToTop20 = &gap
	}
	return position, nil
}

func usageRebateTopLimit() int {
	return len(service.UsageRebateRates())
}

func (r *usageRebateRepository) ListUserRewards(ctx context.Context, userID int64, limit int) ([]service.UsageRebateReward, error) {
	rows, err := r.db.QueryContext(ctx, usageRebateRewardSelect+`
WHERE rr.user_id=$1
ORDER BY rr.business_date DESC, rr.id DESC
LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsageRebateRewards(rows)
}

func (r *usageRebateRepository) ListRecentPeriods(ctx context.Context, limit int) ([]service.UsageRebatePeriod, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, business_date::text, window_start, window_end, settle_after, timezone, rule_version,
       rates, status, total_spend, total_reward, attempt_count, error_message, lock_token,
       locked_until, settled_at, created_at, updated_at
FROM usage_rebate_periods
ORDER BY business_date DESC, id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var periods []service.UsageRebatePeriod
	for rows.Next() {
		period, scanErr := scanUsageRebatePeriod(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		periods = append(periods, *period)
	}
	return periods, rows.Err()
}

func (r *usageRebateRepository) ListPeriodRewards(ctx context.Context, periodID int64, limit int) ([]service.UsageRebateReward, error) {
	rows, err := r.db.QueryContext(ctx, usageRebateRewardSelect+`
WHERE rr.period_id=$1
ORDER BY rr.rank ASC, rr.id ASC
LIMIT $2`, periodID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUsageRebateRewards(rows)
}

type usageRebateQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func queryUsageRebateCandidates(ctx context.Context, queryer usageRebateQueryer, start, end time.Time, viewerUserID int64, limit int) ([]service.UsageRebateCandidate, error) {
	rows, err := queryer.QueryContext(ctx, `
WITH ranked AS (
    SELECT ul.user_id,
           COALESCE(NULLIF(u.username, ''), 'Anonymous') AS username,
           COUNT(*)::bigint AS requests,
           COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS tokens,
           COALESCE(SUM(ul.actual_cost), 0)::numeric(20,8) AS spend_amount,
           ROW_NUMBER() OVER (
               ORDER BY COALESCE(SUM(ul.actual_cost), 0) DESC,
                        COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) DESC,
                        ul.user_id ASC
           )::integer AS rank
    FROM usage_logs ul
    JOIN users u ON u.id=ul.user_id
    WHERE ul.created_at >= $1 AND ul.created_at < $2
      AND ul.billing_type=0 AND ul.actual_cost > 0
    GROUP BY ul.user_id, u.username
)
SELECT user_id, username, requests, tokens, spend_amount, rank
FROM ranked
WHERE rank <= $3 OR ($4 > 0 AND user_id=$4)
ORDER BY rank ASC`, start, end, limit, viewerUserID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var candidates []service.UsageRebateCandidate
	rates := service.UsageRebateRates()
	for rows.Next() {
		var candidate service.UsageRebateCandidate
		if err := rows.Scan(&candidate.UserID, &candidate.Username, &candidate.Requests, &candidate.Tokens, &candidate.SpendAmount, &candidate.Rank); err != nil {
			return nil, err
		}
		if candidate.Rank >= 1 && candidate.Rank <= len(rates) {
			candidate.RebatePercent = rates[candidate.Rank-1].Percent
			candidate.EstimatedReward, _ = service.CalculateUsageRebate(candidate.SpendAmount, candidate.Rank)
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

const usageRebateRewardSelect = `
SELECT rr.id, rr.period_id, rr.business_date::text, rr.user_id,
       COALESCE(NULLIF(u.username, ''), 'Anonymous'),
       rr.rank, rr.spend_amount, rr.rebate_percent, rr.reward_amount, rr.status,
       rr.business_key, rr.error_message, COALESCE(rr.balance_before,0), COALESCE(rr.balance_after,0),
       rr.credited_at, rr.created_at, rr.updated_at
FROM usage_rebate_rewards rr
JOIN users u ON u.id=rr.user_id
`

func scanUsageRebatePeriod(row interface{ Scan(dest ...any) error }) (*service.UsageRebatePeriod, error) {
	var period service.UsageRebatePeriod
	var ratesJSON []byte
	var lockToken sql.NullString
	if err := row.Scan(
		&period.ID, &period.BusinessDate, &period.WindowStart, &period.WindowEnd, &period.SettleAfter,
		&period.Timezone, &period.RuleVersion, &ratesJSON, &period.Status, &period.TotalSpend,
		&period.TotalReward, &period.AttemptCount, &period.ErrorMessage, &lockToken,
		&period.LockedUntil, &period.SettledAt, &period.CreatedAt, &period.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if lockToken.Valid {
		period.SettlementToken = lockToken.String
	}
	if len(ratesJSON) > 0 {
		if err := json.Unmarshal(ratesJSON, &period.Rates); err != nil {
			return nil, err
		}
	}
	return &period, nil
}

func scanUsageRebateRewards(rows *sql.Rows) ([]service.UsageRebateReward, error) {
	var rewards []service.UsageRebateReward
	for rows.Next() {
		var reward service.UsageRebateReward
		if err := rows.Scan(
			&reward.ID, &reward.PeriodID, &reward.BusinessDate, &reward.UserID, &reward.Username,
			&reward.Rank, &reward.SpendAmount, &reward.RebatePercent, &reward.RewardAmount,
			&reward.Status, &reward.BusinessKey, &reward.ErrorMessage, &reward.BalanceBefore,
			&reward.BalanceAfter, &reward.CreditedAt, &reward.CreatedAt, &reward.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, rows.Err()
}

func usageRebateLockToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
