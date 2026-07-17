//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type integrationUsageRebateSettings struct{}

func (integrationUsageRebateSettings) IsUsageRebateEnabled(context.Context) bool { return false }

func TestUsageRebateRepositoryEndToEndWithTwentyFiveUsersAndTwoRunners(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	var userIDs []int64
	var groupID, accountID, apiKeyID int64

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO groups (name) VALUES ($1) RETURNING id`, fmt.Sprintf("usage-rebate-%d", suffix)).Scan(&groupID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, credentials) VALUES ($1,'openai','apikey','{}') RETURNING id`,
		fmt.Sprintf("usage-rebate-%d", suffix)).Scan(&accountID))

	for index := 1; index <= 25; index++ {
		var userID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, username, balance)
VALUES ($1,'test','user-' || $2::text,100) RETURNING id`,
			fmt.Sprintf("usage-rebate-%d-%d@example.invalid", suffix, index), index).Scan(&userID))
		userIDs = append(userIDs, userID)
	}
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO api_keys (user_id, key, name, group_id)
VALUES ($1,$2,'usage-rebate',$3) RETURNING id`, userIDs[0], fmt.Sprintf("sk-usage-rebate-%d", suffix), groupID).Scan(&apiKeyID))

	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	windowStart := time.Now().In(location).AddDate(0, 0, -1).Truncate(24 * time.Hour)
	windowStart = time.Date(windowStart.Year(), windowStart.Month(), windowStart.Day(), 0, 0, 0, 0, location)
	windowEnd := windowStart.AddDate(0, 0, 1)
	for index, userID := range userIDs {
		spend := decimal.NewFromInt(int64(25 - index))
		require.NoError(t, insertUsageRebateIntegrationLog(ctx, userID, apiKeyID, accountID,
			fmt.Sprintf("usage-rebate-%d-%d", suffix, index), spend, service.BillingTypeBalance, windowStart.Add(time.Hour)))
	}
	require.NoError(t, insertUsageRebateIntegrationLog(ctx, userIDs[24], apiKeyID, accountID,
		fmt.Sprintf("usage-rebate-subscription-%d", suffix), decimal.NewFromInt(1000), service.BillingTypeSubscription, windowStart.Add(2*time.Hour)))

	repo := NewUsageRebateRepository(integrationDB)
	require.NoError(t, repo.EnsureOpenPeriod(ctx, service.UsageRebatePeriodSeed{
		BusinessDate: windowStart.Format("2006-01-02"), WindowStart: windowStart, WindowEnd: windowEnd,
		SettleAfter: windowEnd.Add(15 * time.Minute), Timezone: "Asia/Shanghai", RuleVersion: "v1",
	}))
	runnerA := service.NewUsageRebateService(repo, integrationUsageRebateSettings{})
	runnerB := service.NewUsageRebateService(repo, integrationUsageRebateSettings{})
	now := windowEnd.Add(16 * time.Minute)

	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, runner := range []*service.UsageRebateService{runnerA, runnerB} {
		wg.Add(1)
		go func(svc *service.UsageRebateService) {
			defer wg.Done()
			errorsCh <- svc.RunOnce(ctx, now)
		}(runner)
	}
	wg.Wait()
	close(errorsCh)
	for runErr := range errorsCh {
		require.NoError(t, runErr)
	}

	var rewardCount, creditedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(*), COUNT(*) FILTER (WHERE status='credited')
FROM usage_rebate_rewards
WHERE business_date=$1`, windowStart.Format("2006-01-02")).Scan(&rewardCount, &creditedCount))
	require.Equal(t, 20, rewardCount)
	require.Equal(t, 20, creditedCount)

	var firstBalance, twentiethBalance, twentyFirstBalance, twentyFifthBalance decimal.Decimal
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userIDs[0]).Scan(&firstBalance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userIDs[19]).Scan(&twentiethBalance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userIDs[20]).Scan(&twentyFirstBalance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userIDs[24]).Scan(&twentyFifthBalance))
	require.True(t, firstBalance.Equal(decimal.RequireFromString("102.5")))
	require.True(t, twentiethBalance.Equal(decimal.RequireFromString("100.15")))
	require.True(t, twentyFirstBalance.Equal(decimal.NewFromInt(100)))
	require.True(t, twentyFifthBalance.Equal(decimal.NewFromInt(100)))

	require.NoError(t, runnerA.RunOnce(ctx, now.Add(time.Minute)))
	var firstBalanceAfterRetry decimal.Decimal
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userIDs[0]).Scan(&firstBalanceAfterRetry))
	require.True(t, firstBalanceAfterRetry.Equal(firstBalance))

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM usage_rebate_rewards WHERE business_date=$1`, windowStart.Format("2006-01-02"))
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM usage_rebate_periods WHERE business_date=$1`, windowStart.Format("2006-01-02"))
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM usage_logs WHERE request_id LIKE $1`, fmt.Sprintf("usage-rebate-%%%d%%", suffix))
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM api_keys WHERE id=$1`, apiKeyID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, accountID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id=$1`, groupID)
		for _, userID := range userIDs {
			_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
		}
	})
}

func insertUsageRebateIntegrationLog(ctx context.Context, userID, apiKeyID, accountID int64, requestID string, spend decimal.Decimal, billingType int8, createdAt time.Time) error {
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO usage_logs (
    user_id, api_key_id, account_id, request_id, model,
    input_tokens, output_tokens, actual_cost, billing_type, created_at
) VALUES ($1,$2,$3,$4,'test-model',100,50,$5,$6,$7)`,
		userID, apiKeyID, accountID, requestID, spend, billingType, createdAt)
	return err
}
