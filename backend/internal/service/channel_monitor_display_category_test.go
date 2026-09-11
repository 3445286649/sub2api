//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorDisplayCategoryCreateAndUpdate(t *testing.T) {
	params := ChannelMonitorCreateParams{
		Name: "GLM", Provider: MonitorProviderOpenAI, Endpoint: "https://example.com",
		APIKey: "test-key", PrimaryModel: "glm-5.3-flash", IntervalSeconds: 60,
		DisplayCategory: "  智谱 GLM  ",
	}
	repo := &duplicateChannelMonitorRepoStub{}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
	m, err := svc.Create(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, "智谱 GLM", m.DisplayCategory)
	require.Equal(t, "智谱 GLM", repo.created[0].DisplayCategory)
	require.Equal(t, params.Provider, m.Provider)
	require.Equal(t, params.Endpoint, m.Endpoint)
	for _, value := range []string{strings.Repeat("界", 51), "x\ny", "x\x00y", string([]byte{0xff})} {
		params.DisplayCategory = value
		_, err = svc.Create(context.Background(), params)
		require.Error(t, err)
	}
	category := "国产聚合"
	err = applyMonitorUpdate(m, ChannelMonitorUpdateParams{DisplayCategory: &category})
	require.NoError(t, err)
	require.Equal(t, category, m.DisplayCategory)
	require.Equal(t, params.Provider, m.Provider)
	err = applyMonitorUpdate(m, ChannelMonitorUpdateParams{})
	require.NoError(t, err)
	require.Equal(t, category, m.DisplayCategory)
	category = ""
	require.NoError(t, applyMonitorUpdate(m, ChannelMonitorUpdateParams{DisplayCategory: &category}))
	require.Empty(t, m.DisplayCategory)
}
