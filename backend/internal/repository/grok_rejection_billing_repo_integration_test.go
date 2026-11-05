//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGrokRejectionSettlement(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB).(service.GrokRejectionBillingRepository)
	user := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-" + uuid.NewString(), Name: "rejection"})
	account := mustCreateAccount(t, client, &service.Account{Name: uuid.NewString(), Type: service.AccountTypeAPIKey})
	source := "test-" + uuid.NewString()
	cmd := &service.GrokRejectionCommand{SourceID: source, LocalRequestID: uuid.NewString(), Receipt: service.GrokRejectionReceipt{BillingRequestID: "a", BillingStatus: "charged", UpstreamAmount: "0.1", UserAmount: "0.25", Currency: "USD"}, Usage: service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, Model: "grok-4", RequestedModel: "grok-4", RequestType: service.RequestTypeSync}}
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.SettleGrokRejection(ctx, cmd.EventKey()); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assertBalance := func(want string) {
		t.Helper()
		var balance string
		require.NoError(t, integrationDB.QueryRow(`SELECT balance::text FROM users WHERE id=$1`, user.ID).Scan(&balance))
		require.Equal(t, want, balance)
	}
	assertBalance("9.75000000")
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE request_id=$1`, service.GrokRejectionUsageRequestID(source, "a")).Scan(&count))
	require.Equal(t, 1, count)
	var cost, total float64
	var label, mode string
	require.NoError(t, integrationDB.QueryRow(`SELECT actual_cost,total_cost,billing_tier,billing_mode FROM usage_logs WHERE request_id=$1`, service.GrokRejectionUsageRequestID(source, "a")).Scan(&cost, &total, &label, &mode))
	require.Equal(t, 0.25, cost)
	require.Equal(t, 0.1, total)
	require.Equal(t, service.GrokRejectionBillingLabel, label)
	require.Equal(t, service.GrokRejectionBillingMode, mode)
	// Changing config, key or local request must not alter the first price.
	cmd.Receipt.UserAmount = "0.5"
	cmd.Usage.APIKeyID++
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err := repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.75000000")
	cmd.Usage.APIKeyID = key.ID
	// Same receipt id with a different upstream amount is a conflict, not a delta.
	cmd.Receipt.UpstreamAmount = "0.2"
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM grok_rejection_inbox WHERE event_key=$1`, cmd.EventKey()).Scan(&state))
	require.Equal(t, "conflict", state)
	assertBalance("9.75000000")
	// Unconfirmed can be upgraded by a later charged receipt.
	cmd.Receipt = service.GrokRejectionReceipt{BillingRequestID: "b", BillingStatus: "unconfirmed", ReviewReason: "upstream_unconfirmed"}
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.75000000")
	cmd.Receipt = service.GrokRejectionReceipt{BillingRequestID: "b", BillingStatus: "charged", UpstreamAmount: "0.1", UserAmount: "0.1"}
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.65000000")
	// Usage retention does not erase the permanent dedup key.
	_, err = integrationDB.Exec(`DELETE FROM usage_logs WHERE request_id=$1`, service.GrokRejectionUsageRequestID(source, "b"))
	require.NoError(t, err)
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.65000000")
	// Different provider namespaces with the same ID are independently billed.
	cmd.SourceID = source + "-other"
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.55000000")
	// Force a usage insert failure: receipt + wallet update must both roll back.
	cmd.Receipt.BillingRequestID = "rollback"
	cmd.Usage.AccountID = -999999
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.Error(t, err)
	assertBalance("9.55000000")
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM grok_rejection_receipts WHERE source_id=$1 AND billing_request_id='rollback'`, cmd.SourceID).Scan(&count))
	require.Zero(t, count)
	// Restore the fixture and resume through a new repository (process recovery).
	cmd.Usage.AccountID = account.ID
	payload, err := json.Marshal(cmd)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE grok_rejection_inbox SET payload=$2,next_attempt_at=NOW() WHERE event_key=$1`, cmd.EventKey(), payload)
	require.NoError(t, err)
	restarted := NewUsageBillingRepository(client, integrationDB).(service.GrokRejectionBillingRepository)
	pending, err := restarted.PendingGrokRejections(ctx, 100)
	require.NoError(t, err)
	require.Contains(t, pending, cmd.EventKey())
	_, err = restarted.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	assertBalance("9.45000000")
	// Cross-user task queries are rejected even after cache loss.
	require.NoError(t, repo.BindGrokRejectionTask(ctx, source, "video-a", user.ID, key.ID))
	owns, err := repo.OwnsGrokRejectionTask(ctx, source, "video-a", user.ID)
	require.NoError(t, err)
	require.True(t, owns)
	owns, err = repo.OwnsGrokRejectionTask(ctx, source, "video-a", user.ID+1)
	require.NoError(t, err)
	require.False(t, owns)
	require.Error(t, repo.BindGrokRejectionTask(ctx, source, "video-a", user.ID+1, key.ID))
	require.NoError(t, repo.ReplaceGrokTaskNormalBilling(ctx, source, "video-a", user.ID))
	replaced, err := restarted.GrokTaskNormalBillingReplaced(ctx, source, "video-a", user.ID)
	require.NoError(t, err)
	require.True(t, replaced)
	require.Error(t, repo.ReplaceGrokTaskNormalBilling(ctx, source, "video-a", user.ID+1))
	// A receipt already owned by this user cannot be billed to another user.
	cmd.SourceID = source
	cmd.Receipt.BillingRequestID = "a"
	cmd.Receipt.UpstreamAmount = "0.1"
	cmd.Usage.UserID = user.ID + 1
	require.NoError(t, repo.EnqueueGrokRejection(ctx, cmd))
	_, err = repo.SettleGrokRejection(ctx, cmd.EventKey())
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT state FROM grok_rejection_inbox WHERE event_key=$1`, cmd.EventKey()).Scan(&state))
	require.Equal(t, "conflict", state)
	assertBalance("9.45000000")
	t.Log(fmt.Sprintf("verified concurrent dedup, fixed price, conflicts, recovery, retention and task ownership for %s", source))
}
