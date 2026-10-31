//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoCapabilitiesGroupIsolation(t *testing.T) {
	a := *openAIVideoTestAccount()
	a.Status = StatusActive
	a.Schedulable = true
	a.AccountGroups = []AccountGroup{{GroupID: 100}}
	a.Credentials[videoModelCapabilitiesKey] = []VideoModelCapability{{Model: "*", MaxReferenceImages: 30, Resolutions: []string{"4k"}}}
	svc := &GatewayService{accountRepo: newGroupAwareMockRepo([]Account{a}), cfg: testConfig()}
	group := &Group{ID: 100, Platform: PlatformOpenAI, Status: StatusActive}
	profiles, err := svc.VideoCapabilities(context.Background(), group, "public-video")
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	require.Equal(t, 30, profiles[0].MaxReferenceImages)
	require.Empty(t, profiles[0].Model)
	group.ID = 200
	profiles, err = svc.VideoCapabilities(context.Background(), group, "public-video")
	require.NoError(t, err)
	require.Empty(t, profiles)
}
