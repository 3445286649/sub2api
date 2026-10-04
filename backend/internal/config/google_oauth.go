package config

import (
	"fmt"
	"net/url"
	"strings"
)

const GoogleOAuthCallbackPath = "/api/v1/auth/oauth/google/callback"

// GoogleOAuthRedirectURLs validates the opt-in multi-domain configuration.
// An empty result preserves the existing single-callback behavior.
func GoogleOAuthRedirectURLs(cfg EmailOAuthProviderConfig) ([]*url.URL, error) {
	additional := strings.TrimSpace(cfg.AdditionalRedirectURLs)
	if additional == "" {
		return nil, nil
	}
	if len(additional) > 16384 {
		return nil, fmt.Errorf("google additional callback URLs are too long")
	}
	frontend := strings.TrimSpace(cfg.FrontendRedirectURL)
	if frontend == "" {
		frontend = "/auth/oauth/callback"
	}
	f, err := url.Parse(frontend)
	if err != nil || f.IsAbs() || f.Host != "" || f.Fragment != "" ||
		!strings.HasPrefix(f.Path, "/") || strings.HasPrefix(f.Path, "//") ||
		strings.ContainsAny(frontend+f.Path, "\\\r\n") {
		return nil, fmt.Errorf("google multi-domain login requires a same-origin frontend path such as /auth/oauth/callback")
	}
	entries := append([]string{cfg.RedirectURL}, strings.Split(additional, "\n")...)
	urls := make([]*url.URL, 0, len(entries))
	hosts := make(map[string]bool)
	for _, entry := range entries {
		raw := strings.TrimSpace(entry)
		if raw == "" && len(urls) > 0 {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
			u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
			u.EscapedPath() != GoogleOAuthCallbackPath ||
			strings.ContainsAny(u.Host, "*\\ \t\r\n") || (u.Port() != "" && u.Port() != "443") ||
			strings.HasSuffix(u.Host, ":") {
			return nil, fmt.Errorf("google callback URLs must use HTTPS and the exact path %s, without credentials, wildcards, queries, fragments or non-HTTPS ports", GoogleOAuthCallbackPath)
		}
		host := strings.ToLower(u.Hostname())
		if hosts[host] {
			return nil, fmt.Errorf("google callback URLs must have distinct hostnames")
		}
		hosts[host] = true
		urls = append(urls, u)
		if len(urls) > 16 {
			return nil, fmt.Errorf("google login supports at most 16 callback domains")
		}
	}
	return urls, nil
}
