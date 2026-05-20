package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type affiliateRedeemRebateRepoStub struct {
	summaries map[int64]*AffiliateSummary

	accrueRedeemCalls int
	lastInviterID     int64
	lastInviteeID     int64
	lastAmount        float64
	lastFreezeHours   int
	lastRedeemCodeID  int64
	applied           bool
	existingRebate    float64
}

func (s *affiliateRedeemRebateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if summary, ok := s.summaries[userID]; ok {
		cloned := *summary
		return &cloned, nil
	}
	return nil, fmt.Errorf("missing affiliate summary for user %d", userID)
}

func (s *affiliateRedeemRebateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	panic("unexpected GetAffiliateByCode call")
}

func (s *affiliateRedeemRebateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	panic("unexpected BindInviter call")
}

func (s *affiliateRedeemRebateRepoStub) AccrueQuota(context.Context, int64, int64, float64, int, *int64) (bool, error) {
	panic("unexpected AccrueQuota call")
}

func (s *affiliateRedeemRebateRepoStub) AccrueQuotaFromRedeemCode(_ context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceRedeemCodeID int64) (bool, error) {
	s.accrueRedeemCalls++
	s.lastInviterID = inviterID
	s.lastInviteeID = inviteeUserID
	s.lastAmount = amount
	s.lastFreezeHours = freezeHours
	s.lastRedeemCodeID = sourceRedeemCodeID
	return s.applied, nil
}

func (s *affiliateRedeemRebateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return s.existingRebate, nil
}

func (s *affiliateRedeemRebateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (s *affiliateRedeemRebateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (s *affiliateRedeemRebateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (s *affiliateRedeemRebateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (s *affiliateRedeemRebateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (s *affiliateRedeemRebateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (s *affiliateRedeemRebateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (s *affiliateRedeemRebateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (s *affiliateRedeemRebateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (s *affiliateRedeemRebateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (s *affiliateRedeemRebateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (s *affiliateRedeemRebateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	panic("unexpected GetAffiliateUserOverview call")
}

type affiliateRedeemRebateSettingRepoStub struct {
	values map[string]string
}

func (s *affiliateRedeemRebateSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *affiliateRedeemRebateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *affiliateRedeemRebateSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *affiliateRedeemRebateSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *affiliateRedeemRebateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *affiliateRedeemRebateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *affiliateRedeemRebateSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestAccrueInviteRebateForRedeemCode_UsesBaseAmountAndRedeemSource(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	inviteeID := int64(20)
	redeemCodeID := int64(30)
	now := time.Now()
	rate := 12.5

	repo := &affiliateRedeemRebateRepoStub{
		applied: true,
		summaries: map[int64]*AffiliateSummary{
			inviteeID: {UserID: inviteeID, InviterID: &inviterID, CreatedAt: now},
			inviterID: {UserID: inviterID, AffRebateRatePercent: &rate, CreatedAt: now},
		},
	}
	settings := &SettingService{settingRepo: &affiliateRedeemRebateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:             "true",
		SettingKeyAffiliateRebateRate:          "5",
		SettingKeyAffiliateRebateFreezeHours:   "6",
		SettingKeyAffiliateRebateDurationDays:  "0",
		SettingKeyAffiliateRebatePerInviteeCap: "0",
	}}}
	svc := NewAffiliateService(repo, settings, nil, nil)

	rebate, err := svc.AccrueInviteRebateForRedeemCode(ctx, inviteeID, 100, redeemCodeID)

	require.NoError(t, err)
	require.InDelta(t, 12.5, rebate, 1e-9)
	require.Equal(t, 1, repo.accrueRedeemCalls)
	require.Equal(t, inviterID, repo.lastInviterID)
	require.Equal(t, inviteeID, repo.lastInviteeID)
	require.InDelta(t, 12.5, repo.lastAmount, 1e-9)
	require.Equal(t, 6, repo.lastFreezeHours)
	require.Equal(t, redeemCodeID, repo.lastRedeemCodeID)
}

func TestAccrueInviteRebateForRedeemCode_RespectsPerInviteeCap(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	inviteeID := int64(20)
	redeemCodeID := int64(30)
	now := time.Now()

	repo := &affiliateRedeemRebateRepoStub{
		applied:        true,
		existingRebate: 4,
		summaries: map[int64]*AffiliateSummary{
			inviteeID: {UserID: inviteeID, InviterID: &inviterID, CreatedAt: now},
			inviterID: {UserID: inviterID, CreatedAt: now},
		},
	}
	settings := &SettingService{settingRepo: &affiliateRedeemRebateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:             "true",
		SettingKeyAffiliateRebateRate:          "5",
		SettingKeyAffiliateRebateFreezeHours:   "0",
		SettingKeyAffiliateRebateDurationDays:  "0",
		SettingKeyAffiliateRebatePerInviteeCap: "8",
	}}}
	svc := NewAffiliateService(repo, settings, nil, nil)

	rebate, err := svc.AccrueInviteRebateForRedeemCode(ctx, inviteeID, 100, redeemCodeID)

	require.NoError(t, err)
	require.InDelta(t, 4.0, rebate, 1e-9)
	require.Equal(t, 1, repo.accrueRedeemCalls)
	require.InDelta(t, 4.0, repo.lastAmount, 1e-9)
}

func TestAccrueInviteRebateForRedeemCode_SkipsWhenDisabled(t *testing.T) {
	ctx := context.Background()
	repo := &affiliateRedeemRebateRepoStub{
		applied: true,
		summaries: map[int64]*AffiliateSummary{
			20: {UserID: 20, CreatedAt: time.Now()},
		},
	}
	settings := &SettingService{settingRepo: &affiliateRedeemRebateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled: "false",
	}}}
	svc := NewAffiliateService(repo, settings, nil, nil)

	rebate, err := svc.AccrueInviteRebateForRedeemCode(ctx, 20, 100, 30)

	require.NoError(t, err)
	require.Zero(t, rebate)
	require.Zero(t, repo.accrueRedeemCalls)
}
