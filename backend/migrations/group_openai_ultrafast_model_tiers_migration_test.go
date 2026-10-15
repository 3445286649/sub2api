package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupOpenAIUltrafastModelTiersMigration(t *testing.T) {
	content, err := FS.ReadFile("246_group_openai_ultrafast_model_tiers.sql")
	require.NoError(t, err)
	require.Contains(t, string(content), "force_openai_ultrafast BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, string(content), "openai_model_tiers JSONB NOT NULL DEFAULT '{}'::jsonb")
}
