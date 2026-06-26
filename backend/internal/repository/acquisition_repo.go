package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type acquisitionRepository struct {
	db *sql.DB
}

func NewAcquisitionRepository(db *sql.DB) service.AcquisitionRepository {
	return &acquisitionRepository{db: db}
}

func (r *acquisitionRepository) ListCampaigns(ctx context.Context, filter service.AcquisitionCampaignFilter) ([]service.AcquisitionCampaign, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{}
	where := ""
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		where = "WHERE status = $1"
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
SELECT id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
       leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
       settled_at, created_at, updated_at
FROM acquisition_campaigns
%s
ORDER BY starts_at DESC, id DESC
LIMIT $%d`, where, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAcquisitionCampaigns(rows)
}

func (r *acquisitionRepository) GetCampaign(ctx context.Context, id int64) (*service.AcquisitionCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
       leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
       settled_at, created_at, updated_at
FROM acquisition_campaigns
WHERE id = $1`, id)
	return scanAcquisitionCampaign(row)
}

func (r *acquisitionRepository) CreateCampaign(ctx context.Context, input service.AcquisitionCampaignInput) (*service.AcquisitionCampaign, error) {
	shares, prizes, err := acquisitionConfigJSON(input)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO acquisition_campaigns (
    name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
    leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
    created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
RETURNING id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
          leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
          settled_at, created_at, updated_at`,
		input.Name, input.Status, input.StartsAt, input.EndsAt, input.LeaderboardEnabled, input.LotteryEnabled,
		input.LeaderboardPoolUSD, shares, prizes, input.LotterySeed)
	return scanAcquisitionCampaign(row)
}

func (r *acquisitionRepository) UpdateCampaign(ctx context.Context, id int64, input service.AcquisitionCampaignInput) (*service.AcquisitionCampaign, error) {
	shares, prizes, err := acquisitionConfigJSON(input)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE acquisition_campaigns
SET name=$2, status=$3, starts_at=$4, ends_at=$5,
    leaderboard_enabled=$6, lottery_enabled=$7, leaderboard_pool_usd=$8,
    leaderboard_shares=$9, lottery_prize_configs=$10, lottery_seed=$11,
    updated_at=NOW()
WHERE id=$1 AND status IN ('draft','active')
RETURNING id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
          leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
          settled_at, created_at, updated_at`,
		id, input.Name, input.Status, input.StartsAt, input.EndsAt, input.LeaderboardEnabled, input.LotteryEnabled,
		input.LeaderboardPoolUSD, shares, prizes, input.LotterySeed)
	return scanAcquisitionCampaign(row)
}

func (r *acquisitionRepository) ListActiveCampaignsForCompletion(ctx context.Context, completedAt time.Time) ([]service.AcquisitionCampaign, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
       leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
       settled_at, created_at, updated_at
FROM acquisition_campaigns
WHERE status = 'active' AND starts_at <= $1 AND ends_at > $1
ORDER BY starts_at DESC, id DESC`, completedAt)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAcquisitionCampaigns(rows)
}

func (r *acquisitionRepository) GetInviteBinding(ctx context.Context, inviteeUserID int64) (*service.AcquisitionInviteBinding, error) {
	var out service.AcquisitionInviteBinding
	err := r.db.QueryRowContext(ctx, `
SELECT inviter_id, created_at
FROM user_affiliates
WHERE user_id = $1 AND inviter_id IS NOT NULL`, inviteeUserID).Scan(&out.InviterID, &out.BoundAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *acquisitionRepository) HasPriorEligiblePayment(ctx context.Context, userID, orderID int64, completedAt time.Time) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM payment_orders
    WHERE user_id = $1
      AND id <> $2
      AND status = 'COMPLETED'
      AND pay_amount > 0
      AND refund_amount = 0
      AND completed_at IS NOT NULL
      AND completed_at <= $3
      AND (
        order_type = 'balance'
        OR (order_type = 'subscription' AND COALESCE(subscription_plan_type, 'subscription') <> 'quota_reset')
      )
)`, userID, orderID, completedAt).Scan(&exists)
	return exists, err
}

func (r *acquisitionRepository) CreateParticipationWithTickets(ctx context.Context, p service.AcquisitionParticipation) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var pid int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO acquisition_participations (campaign_id, inviter_id, invitee_id, source_order_id, completed_at, created_at)
VALUES ($1,$2,$3,$4,$5,NOW())
ON CONFLICT (campaign_id, invitee_id) DO NOTHING
RETURNING id`, p.CampaignID, p.InviterID, p.InviteeID, p.SourceOrderID, p.CompletedAt).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	for _, ticket := range []struct {
		userID int64
		role   string
	}{
		{p.InviterID, service.AcquisitionTicketRoleInviter},
		{p.InviteeID, service.AcquisitionTicketRoleInvitee},
	} {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO acquisition_lottery_tickets (campaign_id, user_id, invitee_id, source_order_id, ticket_role, created_at)
VALUES ($1,$2,$3,$4,$5,NOW())
ON CONFLICT (campaign_id, source_order_id, ticket_role) DO NOTHING`,
			p.CampaignID, ticket.userID, p.InviteeID, p.SourceOrderID, ticket.role); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (r *acquisitionRepository) ClaimDueCampaign(ctx context.Context, now time.Time) (*service.AcquisitionCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
UPDATE acquisition_campaigns
SET status = 'settling', updated_at = NOW()
WHERE id = (
    SELECT id
    FROM acquisition_campaigns
    WHERE status IN ('active','settling') AND ends_at <= $1
    ORDER BY ends_at ASC, id ASC
    LIMIT 1
)
RETURNING id, name, status, starts_at, ends_at, leaderboard_enabled, lottery_enabled,
          leaderboard_pool_usd, leaderboard_shares, lottery_prize_configs, lottery_seed,
          settled_at, created_at, updated_at`, now)
	campaign, err := scanAcquisitionCampaign(row)
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, service.ErrAcquisitionNotFound) {
		return nil, nil
	}
	return campaign, err
}

func (r *acquisitionRepository) ListLeaderboard(ctx context.Context, campaignID int64) ([]service.AcquisitionLeaderboardRow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT p.inviter_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       COUNT(*)::integer AS invite_count,
       MAX(p.completed_at) AS last_completed_at
FROM acquisition_participations p
JOIN users u ON u.id = p.inviter_id
WHERE p.campaign_id = $1
GROUP BY p.inviter_id, u.email, u.username
ORDER BY invite_count DESC, last_completed_at ASC, p.inviter_id ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.AcquisitionLeaderboardRow
	rank := 1
	for rows.Next() {
		var row service.AcquisitionLeaderboardRow
		var last time.Time
		if err := rows.Scan(&row.UserID, &row.Email, &row.Username, &row.InviteCount, &last); err != nil {
			return nil, err
		}
		row.LastCompletedAt = &last
		row.Rank = rank
		rank++
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *acquisitionRepository) ListLotteryTickets(ctx context.Context, campaignID int64) ([]service.AcquisitionLotteryTicket, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, campaign_id, user_id, invitee_id, source_order_id, ticket_role, created_at
FROM acquisition_lottery_tickets
WHERE campaign_id = $1
ORDER BY id ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.AcquisitionLotteryTicket
	for rows.Next() {
		var ticket service.AcquisitionLotteryTicket
		if err := rows.Scan(&ticket.ID, &ticket.CampaignID, &ticket.UserID, &ticket.InviteeID, &ticket.SourceOrderID, &ticket.TicketRole, &ticket.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ticket)
	}
	return out, rows.Err()
}

func (r *acquisitionRepository) CreateReward(ctx context.Context, reward service.AcquisitionReward) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO acquisition_rewards (campaign_id, user_id, reward_type, reward_key, amount, rank, prize_name, ticket_id, status, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',NOW(),NOW())
ON CONFLICT (campaign_id, reward_type, reward_key) DO NOTHING`,
		reward.CampaignID, reward.UserID, reward.RewardType, reward.RewardKey, reward.Amount, reward.Rank, reward.PrizeName, reward.TicketID)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (r *acquisitionRepository) ListPendingRewards(ctx context.Context, campaignID int64) ([]service.AcquisitionReward, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, campaign_id, user_id, reward_type, reward_key, amount, rank, prize_name, ticket_id, status, error_message, paid_at, created_at
FROM acquisition_rewards
WHERE campaign_id = $1 AND status IN ('pending','failed')
ORDER BY id ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAcquisitionRewards(rows)
}

func (r *acquisitionRepository) PayReward(ctx context.Context, reward service.AcquisitionReward) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var userID int64
	var amount float64
	err = tx.QueryRowContext(ctx, `
SELECT status, user_id, amount
FROM acquisition_rewards
WHERE id = $1
FOR UPDATE`, reward.ID).Scan(&status, &userID, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == service.AcquisitionRewardStatusPaid {
		return tx.Commit()
	}
	if status != service.AcquisitionRewardStatusPending && status != service.AcquisitionRewardStatusFailed {
		return tx.Commit()
	}
	if amount <= 0 {
		return tx.Commit()
	}

	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `
UPDATE users
SET balance = balance + $2, updated_at = NOW()
WHERE id = $1
RETURNING balance`, userID, amount).Scan(&balanceAfter); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE acquisition_rewards
SET status='paid', error_message='', paid_at=NOW(), balance_after=$2, updated_at=NOW()
WHERE id=$1 AND status IN ('pending','failed')`, reward.ID, balanceAfter); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *acquisitionRepository) MarkRewardFailed(ctx context.Context, rewardID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE acquisition_rewards
SET status='failed', error_message=$2, updated_at=NOW()
WHERE id=$1 AND status <> 'paid'`, rewardID, reason)
	return err
}

func (r *acquisitionRepository) MarkCampaignSettled(ctx context.Context, campaignID int64, settledAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE acquisition_campaigns
SET status='settled', settled_at=$2, updated_at=NOW()
WHERE id=$1`, campaignID, settledAt)
	return err
}

func (r *acquisitionRepository) GetUserSummary(ctx context.Context, campaignID, userID int64) (*service.AcquisitionUserSummary, error) {
	campaign, err := r.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	leaderboard, err := r.ListLeaderboard(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	var validInvites, ticketCount, rank int
	for _, row := range leaderboard {
		if row.UserID == userID {
			validInvites = row.InviteCount
			rank = row.Rank
			break
		}
	}
	if userID > 0 {
		_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM acquisition_lottery_tickets WHERE campaign_id=$1 AND user_id=$2`, campaignID, userID).Scan(&ticketCount)
	}
	rewards, err := r.ListRewards(ctx, campaignID, nullableUserFilter(userID))
	if err != nil {
		return nil, err
	}
	var affCode string
	if userID > 0 {
		_ = r.db.QueryRowContext(ctx, `SELECT aff_code FROM user_affiliates WHERE user_id=$1`, userID).Scan(&affCode)
	}
	return &service.AcquisitionUserSummary{
		Campaign:     campaign,
		AffCode:      affCode,
		ValidInvites: validInvites,
		Rank:         rank,
		TicketCount:  ticketCount,
		Leaderboard:  leaderboard,
		Rewards:      rewards,
	}, nil
}

func (r *acquisitionRepository) ListRewards(ctx context.Context, campaignID int64, userID *int64) ([]service.AcquisitionReward, error) {
	args := []any{campaignID}
	where := "campaign_id = $1"
	if userID != nil && *userID > 0 {
		args = append(args, *userID)
		where += " AND user_id = $2"
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
SELECT id, campaign_id, user_id, reward_type, reward_key, amount, rank, prize_name, ticket_id, status, error_message, paid_at, created_at
FROM acquisition_rewards
WHERE %s
ORDER BY created_at DESC, id DESC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAcquisitionRewards(rows)
}

func acquisitionConfigJSON(input service.AcquisitionCampaignInput) ([]byte, []byte, error) {
	shares, err := json.Marshal(input.LeaderboardShares)
	if err != nil {
		return nil, nil, err
	}
	prizes, err := json.Marshal(input.LotteryPrizeConfigs)
	if err != nil {
		return nil, nil, err
	}
	return shares, prizes, nil
}

func scanAcquisitionCampaign(row interface{ Scan(dest ...any) error }) (*service.AcquisitionCampaign, error) {
	var campaign service.AcquisitionCampaign
	var shares, prizes []byte
	if err := row.Scan(
		&campaign.ID, &campaign.Name, &campaign.Status, &campaign.StartsAt, &campaign.EndsAt,
		&campaign.LeaderboardEnabled, &campaign.LotteryEnabled, &campaign.LeaderboardPoolUSD,
		&shares, &prizes, &campaign.LotterySeed, &campaign.SettledAt, &campaign.CreatedAt, &campaign.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAcquisitionNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(shares, &campaign.LeaderboardShares)
	_ = json.Unmarshal(prizes, &campaign.LotteryPrizeConfigs)
	return &campaign, nil
}

func scanAcquisitionCampaigns(rows *sql.Rows) ([]service.AcquisitionCampaign, error) {
	var out []service.AcquisitionCampaign
	for rows.Next() {
		campaign, err := scanAcquisitionCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *campaign)
	}
	return out, rows.Err()
}

func scanAcquisitionRewards(rows *sql.Rows) ([]service.AcquisitionReward, error) {
	var out []service.AcquisitionReward
	for rows.Next() {
		var reward service.AcquisitionReward
		if err := rows.Scan(
			&reward.ID, &reward.CampaignID, &reward.UserID, &reward.RewardType, &reward.RewardKey,
			&reward.Amount, &reward.Rank, &reward.PrizeName, &reward.TicketID, &reward.Status,
			&reward.ErrorMessage, &reward.PaidAt, &reward.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, reward)
	}
	return out, rows.Err()
}

func nullableUserFilter(userID int64) *int64 {
	if userID <= 0 {
		return nil
	}
	return &userID
}
