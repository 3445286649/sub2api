package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type balanceForecastRepoFake struct {
	closeCalls int
	listCalls  int
	openCalls  int
	episode    *BalanceForecastEpisode
	notified   []int64
}

func (r *balanceForecastRepoFake) CloseRecoveredEpisodes(context.Context, time.Time) (int64, error) {
	r.closeCalls++
	return 0, nil
}

func (r *balanceForecastRepoFake) ListCandidates(context.Context, time.Time, float64) ([]BalanceForecastCandidate, error) {
	r.listCalls++
	return nil, nil
}

func (r *balanceForecastRepoFake) OpenEpisode(context.Context, BalanceForecastEpisodeInput) (*BalanceForecastEpisode, error) {
	r.openCalls++
	return r.episode, nil
}

func (r *balanceForecastRepoFake) MarkEpisodeNotified(_ context.Context, id int64, _ time.Time) error {
	r.notified = append(r.notified, id)
	return nil
}

func TestComputeBalanceForecast(t *testing.T) {
	cases := []struct {
		name       string
		balance    float64
		windowCost float64
		wantOK     bool
		wantHours  float64
	}{
		{"burns through in a day", 10, 70, true, 24},
		{"exactly on the 72 hour horizon", 30, 70, true, 72},
		{"just past the horizon", 31, 70, false, 0},
		{"zero balance is not forecast", 0, 70, false, 0},
		{"no spending in window", 10, 0, false, 0},
		{"spending below the noise floor", 0.01, 0.01, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := computeBalanceForecast(tc.balance, tc.windowCost)
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.InDelta(t, tc.wantHours, got.HoursRemaining, 0.001)
				require.InDelta(t, tc.windowCost/7, got.DailyBurn, 0.001)
			}
		})
	}
}

func TestBalanceForecastService_EnabledDefaultsToFalse(t *testing.T) {
	svc := NewBalanceForecastService(nil, time.Minute)
	require.False(t, svc.enabled(context.Background()), "no setting repository")

	svc.SetSettingRepository(&subscriptionExpirySettingRepoStub{values: map[string]string{}})
	require.False(t, svc.enabled(context.Background()), "setting missing")
}

func TestBalanceForecastService_EnabledOnlyWhenExplicitlyTrue(t *testing.T) {
	for value, want := range map[string]bool{"true": true, "false": false, "1": false, "": false} {
		svc := NewBalanceForecastService(nil, time.Minute)
		svc.SetSettingRepository(&subscriptionExpirySettingRepoStub{
			values: map[string]string{SettingKeyBalanceForecastNotifyEnabled: value},
		})
		require.Equal(t, want, svc.enabled(context.Background()), "value %q", value)
	}
}

func TestBalanceForecastService_SettingReadErrorFailsClosed(t *testing.T) {
	svc := NewBalanceForecastService(nil, time.Minute)
	svc.SetSettingRepository(&subscriptionExpirySettingRepoStub{err: errors.New("db down")})
	require.False(t, svc.enabled(context.Background()))
}

func TestBalanceForecastService_DisabledDoesNotTouchRepository(t *testing.T) {
	repo := &balanceForecastRepoFake{}
	settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{}}
	svc := NewBalanceForecastService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, nil))

	svc.run(context.Background())

	require.Zero(t, repo.closeCalls)
	require.Zero(t, repo.listCalls)
}

func TestBalanceForecastService_NoVerifiedRecipientDoesNotOpenEpisode(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":      "[]",
		"unverified": `[{"email":"a@example.com","disabled":false,"verified":false}]`,
		"disabled":   `[{"email":"a@example.com","disabled":true,"verified":true}]`,
	} {
		t.Run(name, func(t *testing.T) {
			repo := &balanceForecastRepoFake{episode: &BalanceForecastEpisode{ID: 1}}
			svc := NewBalanceForecastService(repo, time.Minute)
			svc.SetNotificationEmailService(NewNotificationEmailService(&subscriptionExpirySettingRepoStub{values: map[string]string{}}, nil))

			svc.forecastOne(context.Background(), &BalanceForecastCandidate{
				UserID: 1, Balance: 10, WindowCost: 70, ExtraEmails: raw,
			}, "", time.Now())

			require.Zero(t, repo.openCalls)
			require.Empty(t, repo.notified)
		})
	}
}

func TestBalanceForecastService_NoForecastDoesNotOpenEpisode(t *testing.T) {
	repo := &balanceForecastRepoFake{episode: &BalanceForecastEpisode{ID: 1}}
	svc := NewBalanceForecastService(repo, time.Minute)
	svc.SetNotificationEmailService(NewNotificationEmailService(&subscriptionExpirySettingRepoStub{values: map[string]string{}}, nil))

	// 余额够撑一个月，不在 72 小时提醒范围内。
	svc.forecastOne(context.Background(), &BalanceForecastCandidate{
		UserID: 1, Balance: 1000, WindowCost: 70,
		ExtraEmails: `[{"email":"a@example.com","disabled":false,"verified":true}]`,
	}, "", time.Now())

	require.Zero(t, repo.openCalls)
}

func TestBalanceForecastService_AlreadyNotifiedEpisodeSendsNothing(t *testing.T) {
	notifiedAt := time.Now().Add(-time.Hour)
	repo := &balanceForecastRepoFake{episode: &BalanceForecastEpisode{ID: 7, NotifiedAt: &notifiedAt}}
	svc := NewBalanceForecastService(repo, time.Minute)
	// 邮件服务没有配置发信通道：若走到发送会失败，而这一轮已通知，不应走到发送。
	svc.SetNotificationEmailService(NewNotificationEmailService(&subscriptionExpirySettingRepoStub{values: map[string]string{}}, nil))

	svc.forecastOne(context.Background(), &BalanceForecastCandidate{
		UserID: 1, Balance: 10, WindowCost: 70,
		ExtraEmails: `[{"email":"a@example.com","disabled":false,"verified":true}]`,
	}, "", time.Now())

	require.Equal(t, 1, repo.openCalls)
	require.Empty(t, repo.notified, "an already notified episode must not be marked again")
}
