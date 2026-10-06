package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePaymentReturnOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "production", raw: "https://codeplan.codes", want: "https://codeplan.codes"},
		{name: "secondary domain", raw: "https://subapi.loucer.cn", want: "https://subapi.loucer.cn"},
		{name: "test domain", raw: "https://us.codeplan.codes", want: "https://us.codeplan.codes"},
		{name: "normalized origin", raw: " HTTPS://CodePlan.Codes:443/ ", want: "https://codeplan.codes"},
		{name: "localhost", raw: "http://localhost:18080", want: "http://localhost:18080"},
		{name: "ipv4 loopback", raw: "http://127.0.0.1:18080/", want: "http://127.0.0.1:18080"},
		{name: "ipv6 loopback", raw: "http://[::1]:18080", want: "http://[::1]:18080"},
		{name: "default local port", raw: "http://localhost:80", want: "http://localhost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ValidatePaymentReturnOrigin(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, u.String())
		})
	}
	for _, raw := range []string{
		"", "https://", "//codeplan.codes", "https:codeplan.codes", "ftp://codeplan.codes",
		"http://codeplan.codes", "http://localhost.evil.example", "http://192.168.1.1",
		"https://user@codeplan.codes", "https://*.codeplan.codes", "https://codeplan.codes/payment/result",
		"https://codeplan.codes/%2f", "https://codeplan.codes?", "https://codeplan.codes?trade_status=TRADE_SUCCESS",
		"https://codeplan.codes#", "https://codeplan.codes#fragment", "https://codeplan.codes:0",
		"https://codeplan.codes:65536", "https://codeplan.codes:", "https://[invalid]",
		"https://codeplan.codes&trade_status=TRADE_SUCCESS", "https://codeplan.codes\\evil.example",
	} {
		t.Run("reject "+raw, func(t *testing.T) {
			u, err := ValidatePaymentReturnOrigin(raw)
			require.Error(t, err)
			require.Nil(t, u)
		})
	}
}

func TestLoadPaymentReturnOrigins(t *testing.T) {
	t.Run("empty by default", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.Server.PaymentReturnOrigins)
	})
	t.Run("configuration normalized", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("server:\n  payment_return_origins:\n    - https://CodePlan.Codes:443/\n    - https://subapi.loucer.cn\n    - https://codeplan.codes\n"), 0o600))
		t.Setenv("CONFIG_FILE", configFile)
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{"https://codeplan.codes", "https://subapi.loucer.cn"}, cfg.Server.PaymentReturnOrigins)
	})
	t.Run("space separated environment overrides configuration", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("server:\n  payment_return_origins:\n    - https://codeplan.codes\n"), 0o600))
		t.Setenv("CONFIG_FILE", configFile)
		t.Setenv("SERVER_PAYMENT_RETURN_ORIGINS", " https://us.codeplan.codes/   http://localhost:18080 ")
		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, []string{"https://us.codeplan.codes", "http://localhost:18080"}, cfg.Server.PaymentReturnOrigins)
	})
	t.Run("explicit empty environment disables origins", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("server:\n  payment_return_origins:\n    - https://codeplan.codes\n"), 0o600))
		t.Setenv("CONFIG_FILE", configFile)
		t.Setenv("SERVER_PAYMENT_RETURN_ORIGINS", "")
		cfg, err := Load()
		require.NoError(t, err)
		require.Empty(t, cfg.Server.PaymentReturnOrigins)
	})
	t.Run("invalid origin prevents startup", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		t.Setenv("SERVER_PAYMENT_RETURN_ORIGINS", "https://us.codeplan.codes?trade_status=TRADE_SUCCESS")
		_, err := Load()
		require.ErrorContains(t, err, "server.payment_return_origins")
	})
	t.Run("invalid YAML empty entry prevents startup", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		configFile := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("server:\n  payment_return_origins:\n    - \"\"\n"), 0o600))
		t.Setenv("CONFIG_FILE", configFile)
		_, err := Load()
		require.ErrorContains(t, err, "server.payment_return_origins")
	})
}
