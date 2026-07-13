package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

func validateGrokThirdPartyAPIKeyBaseURL(raw string) (string, error) {
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		RequireAllowlist: false,
		AllowPrivate:     false,
	})
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
	if account != nil && account.Platform == PlatformGrok && account.Type == AccountTypeAPIKey {
		return urlvalidator.WithPublicIPValidation(ctx)
	}
	return ctx
}

func buildGrokAnthropicMessagesURL(base string) string {
	return buildOpenAIEndpointURL(strings.TrimSpace(base), "/v1/messages") + "?beta=true"
}
