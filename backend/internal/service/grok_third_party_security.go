package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

func validateGrokThirdPartyAPIKeyBaseURL(raw string, cfg *config.Config) (string, error) {
	allowInsecureHTTP := cfg != nil && cfg.Security.URLAllowlist.AllowInsecureHTTP
	options := urlvalidator.ValidationOptions{}
	if cfg != nil && cfg.Security.URLAllowlist.Enabled {
		options.AllowedHosts = cfg.Security.URLAllowlist.UpstreamHosts
		options.RequireAllowlist = true
		options.AllowPrivate = cfg.Security.URLAllowlist.AllowPrivateHosts
	}
	normalized, err := urlvalidator.ValidateHTTPURL(raw, allowInsecureHTTP, options)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if parsed.User != nil {
		return "", errors.New("base URL must not contain userinfo")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not contain query or fragment")
	}
	return normalized, nil
}

func withGrokThirdPartyPublicNetworkValidation(ctx context.Context, account *Account) context.Context {
	// Resolved-IP validation follows the global URL allowlist policy in HTTPUpstream.
	return ctx
}

func buildGrokAnthropicMessagesURL(base string) string {
	return buildOpenAIEndpointURL(strings.TrimSpace(base), "/v1/messages") + "?beta=true"
}
