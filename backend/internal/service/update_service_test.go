package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestNormalizeVersionTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain semver",
			in:   "0.1.125",
			want: "0.1.125",
		},
		{
			name: "tag prefix",
			in:   "v0.1.125",
			want: "0.1.125",
		},
		{
			name: "local build suffix",
			in:   "0.1.125-loucer-20260508-075819",
			want: "0.1.125",
		},
		{
			name: "metadata suffix",
			in:   "0.1.125+loucer",
			want: "0.1.125",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeVersionTag(tt.in); got != tt.want {
				t.Fatalf("normalizeVersionTag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCompareVersionsIgnoresLocalBuildSuffix(t *testing.T) {
	if got := compareVersions("0.1.125-loucer-20260508-075819", "0.1.125"); got != 0 {
		t.Fatalf("compareVersions with local suffix = %d, want 0", got)
	}
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.133",
				Name:    "v0.1.133",
			},
		},
		"0.1.133",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}
