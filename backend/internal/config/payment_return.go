package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// ValidatePaymentReturnOrigin accepts only exact site origins, never client redirect URLs.
func ValidatePaymentReturnOrigin(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || raw == "" || u.Hostname() == "" || u.Opaque != "" ||
		(u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") ||
		(u.EscapedPath() != "" && u.EscapedPath() != "/") ||
		strings.ContainsAny(u.Host, "*\\ \t\r\n&=") || strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("must be an exact HTTP(S) origin without credentials, wildcards, paths, queries or fragments")
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if ip != nil {
		host = ip.String()
	} else {
		if strings.ContainsAny(host, "[]:") {
			return nil, fmt.Errorf("origin hostname is invalid")
		}
		host, err = idna.Lookup.ToASCII(host)
		if err != nil || host == "" {
			return nil, fmt.Errorf("origin hostname is invalid")
		}
	}
	if u.Scheme == "http" && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("must use HTTPS; HTTP is allowed only for localhost or loopback addresses")
	}
	port := u.Port()
	if port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, fmt.Errorf("origin port must be between 1 and 65535")
		}
		port = strconv.Itoa(value)
		if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
			port = ""
		}
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return &url.URL{Scheme: u.Scheme, Host: host}, nil
}

func normalizePaymentReturnOrigins(origins []string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	seen := make(map[string]bool, len(origins))
	for _, origin := range origins {
		u, err := ValidatePaymentReturnOrigin(origin)
		if err != nil {
			return nil, err
		}
		value := u.String()
		if !seen[value] {
			normalized = append(normalized, value)
			seen[value] = true
		}
	}
	return normalized, nil
}
