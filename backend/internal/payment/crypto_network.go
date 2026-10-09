package payment

import (
	"context"
	"strings"
)

type cryptoNetworkContextKey struct{}

func WithCryptoNetwork(ctx context.Context, network string) context.Context {
	return context.WithValue(ctx, cryptoNetworkContextKey{}, network)
}
func cryptoNetworkFromContext(ctx context.Context) string {
	network, _ := ctx.Value(cryptoNetworkContextKey{}).(string)
	return network
}
func cryptoNetworkAllowed(config map[string]string, network string) bool {
	if network == "" {
		return false
	}
	for _, entry := range strings.Split(config["allowedNetworks"], ",") {
		if strings.TrimSpace(entry) == network {
			return true
		}
	}
	return false
}
