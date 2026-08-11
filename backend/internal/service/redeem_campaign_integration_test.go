package service_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type redeemCampaignTestHarness struct {
	client  *dbent.Client
	service *service.RedeemService
	adminID int64
}

func newRedeemCampaignTestHarness(t *testing.T) *redeemCampaignTestHarness {
	t.Helper()

	databasePath := filepath.ToSlash(filepath.Join(t.TempDir(), "redeem-campaign.db"))
	db, err := sql.Open("sqlite", "file:"+databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	require.NoError(t, err)
	db.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = db.Close() })

	driver := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(driver)))
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	admin, err := client.User.Create().
		SetEmail("campaign-admin@example.com").
		SetPasswordHash("hash").
		SetRole(service.RoleAdmin).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	redeemRepo := repository.NewRedeemCodeRepository(client)
	userRepo := repository.NewUserRepository(client, db)
	return &redeemCampaignTestHarness{
		client:  client,
		service: service.NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil),
		adminID: admin.ID,
	}
}

func (h *redeemCampaignTestHarness) createUser(t *testing.T, email string) int64 {
	t.Helper()
	user, err := h.client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(context.Background())
	require.NoError(t, err)
	return user.ID
}

func (h *redeemCampaignTestHarness) generateCampaign(t *testing.T, name string, count int, value float64) []service.RedeemCode {
	t.Helper()
	codes, err := h.service.GenerateRedeemCampaignCodes(context.Background(), service.GenerateRedeemCampaignCodesInput{
		Name:      name,
		Count:     count,
		Value:     value,
		CreatedBy: h.adminID,
	})
	require.NoError(t, err)
	require.Len(t, codes, count)
	return codes
}

func TestRedeemCampaignLimitsEachUserOnceWithoutAffectingOtherCodes(t *testing.T) {
	h := newRedeemCampaignTestHarness(t)
	ctx := context.Background()
	userOne := h.createUser(t, "campaign-user-one@example.com")
	userTwo := h.createUser(t, "campaign-user-two@example.com")

	firstCampaign := h.generateCampaign(t, "campaign-one", 2, 10)
	_, err := h.service.Redeem(ctx, userOne, firstCampaign[0].Code)
	require.NoError(t, err)

	_, err = h.service.Redeem(ctx, userOne, firstCampaign[1].Code)
	require.ErrorIs(t, err, service.ErrRedeemCampaignUsed)
	require.Equal(t, "REDEEM_CAMPAIGN_ALREADY_REDEEMED", infraerrors.Reason(err))

	unusedCode, err := h.client.RedeemCode.Get(ctx, firstCampaign[1].ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, unusedCode.Status)
	require.Nil(t, unusedCode.UsedBy)

	_, err = h.service.Redeem(ctx, userTwo, firstCampaign[1].Code)
	require.NoError(t, err)

	secondCampaign := h.generateCampaign(t, "campaign-two", 1, 5)
	_, err = h.service.Redeem(ctx, userOne, secondCampaign[0].Code)
	require.NoError(t, err)

	for _, code := range []string{"ordinary-one", "ordinary-two"} {
		err = h.service.CreateCode(ctx, &service.RedeemCode{
			Code:   code,
			Type:   service.RedeemTypeBalance,
			Value:  2,
			Status: service.StatusUnused,
		})
		require.NoError(t, err)
		_, err = h.service.Redeem(ctx, userOne, code)
		require.NoError(t, err)
	}

	updatedUser, err := h.client.User.Get(ctx, userOne)
	require.NoError(t, err)
	require.InDelta(t, 19, updatedUser.Balance, 0.000001)
}

func TestRedeemCampaignConcurrentCodesCreditBalanceOnce(t *testing.T) {
	h := newRedeemCampaignTestHarness(t)
	ctx := context.Background()
	userID := h.createUser(t, "campaign-concurrent@example.com")
	codes := h.generateCampaign(t, "campaign-concurrent", 2, 10)

	start := make(chan struct{})
	errorsByCode := make([]error, len(codes))
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, errorsByCode[index] = h.service.Redeem(ctx, userID, codes[index].Code)
		}(i)
	}
	close(start)
	wg.Wait()

	var successes, campaignConflicts int
	for _, err := range errorsByCode {
		switch {
		case err == nil:
			successes++
		case infraerrors.Reason(err) == "REDEEM_CAMPAIGN_ALREADY_REDEEMED":
			campaignConflicts++
		default:
			t.Fatalf("unexpected concurrent redeem error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, campaignConflicts)

	updatedUser, err := h.client.User.Get(ctx, userID)
	require.NoError(t, err)
	require.InDelta(t, 10, updatedUser.Balance, 0.000001)

	usedCount, err := h.client.RedeemCode.Query().
		Where(redeemcode.CampaignIDEQ(*codes[0].CampaignID), redeemcode.StatusEQ(service.StatusUsed)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, usedCount)

	redemptionCount, err := h.client.RedeemCampaignRedemption.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, redemptionCount)
}
