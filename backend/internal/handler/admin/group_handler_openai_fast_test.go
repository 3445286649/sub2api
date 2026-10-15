package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupRequestsDecodeForceOpenAIFast(t *testing.T) {
	var createReq CreateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"fast","force_openai_fast":true,"force_openai_ultrafast":true,"openai_model_tiers":{"gpt-6-astra":"ultrafast"},"free_openai_fast":true}`), &createReq))
	require.True(t, createReq.ForceOpenAIFast)
	require.True(t, createReq.ForceOpenAIUltrafast)
	require.Equal(t, "ultrafast", createReq.OpenAIModelTiers["gpt-6-astra"])
	require.True(t, createReq.FreeOpenAIFast)

	var updateReq UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"force_openai_fast":false,"force_openai_ultrafast":false,"openai_model_tiers":{},"free_openai_fast":false}`), &updateReq))
	require.NotNil(t, updateReq.ForceOpenAIFast)
	require.False(t, *updateReq.ForceOpenAIFast)
	require.NotNil(t, updateReq.ForceOpenAIUltrafast)
	require.False(t, *updateReq.ForceOpenAIUltrafast)
	require.NotNil(t, updateReq.OpenAIModelTiers)
	require.NotNil(t, updateReq.FreeOpenAIFast)
	require.False(t, *updateReq.FreeOpenAIFast)

	var omitted UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.ForceOpenAIFast)
	require.Nil(t, omitted.ForceOpenAIUltrafast)
	require.Nil(t, omitted.OpenAIModelTiers)
	require.Nil(t, omitted.FreeOpenAIFast)
}
