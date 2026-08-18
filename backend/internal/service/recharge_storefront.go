package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

const (
	maxRechargeStorefrontChannels    = 8
	maxRechargeStorefrontChannelID   = 64
	maxRechargeStorefrontChannelName = 64
	maxRechargeStorefrontURLLength   = 2048
	defaultRechargeStorefrontURL     = "https://shop.loucer.cn/"
	defaultRechargeStorefrontBackup  = "https://pay.ldxp.cn/shop/9QURDNRF"
	defaultRechargeStorefrontJSON    = `[{"id":"backup-1","name":"备用一","url":"https://shop.loucer.cn/","enabled":true,"sort_order":1},{"id":"backup-2","name":"备用二","url":"https://pay.ldxp.cn/shop/9QURDNRF","enabled":true,"sort_order":2}]`
)

// RechargeStorefrontChannel is the admin/public representation of an external
// recharge storefront. The same shape is persisted as JSON in settings.
type RechargeStorefrontChannel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
}

// NormalizeRechargeStorefrontChannels validates and returns a stable ordered
// copy. It is intentionally shared by admin writes and persisted-value reads.
func NormalizeRechargeStorefrontChannels(channels []RechargeStorefrontChannel) ([]RechargeStorefrontChannel, error) {
	if len(channels) > maxRechargeStorefrontChannels {
		return nil, fmt.Errorf("too many recharge storefront channels: maximum is %d", maxRechargeStorefrontChannels)
	}

	seenIDs := make(map[string]struct{}, len(channels))
	seenURLs := make(map[string]struct{}, len(channels))
	normalized := make([]RechargeStorefrontChannel, 0, len(channels))
	for index, channel := range channels {
		channel.ID = strings.TrimSpace(channel.ID)
		if channel.ID == "" {
			channel.ID = fmt.Sprintf("backup-%d", index+1)
		}
		if len(channel.ID) > maxRechargeStorefrontChannelID || !isSafeRechargeStorefrontID(channel.ID) {
			return nil, fmt.Errorf("invalid recharge storefront channel id %q", channel.ID)
		}
		if _, exists := seenIDs[channel.ID]; exists {
			return nil, fmt.Errorf("duplicate recharge storefront channel id %q", channel.ID)
		}
		seenIDs[channel.ID] = struct{}{}

		channel.Name = strings.TrimSpace(channel.Name)
		if channel.Name == "" {
			channel.Name = fmt.Sprintf("备用%d", index+1)
		}
		if len(channel.Name) > maxRechargeStorefrontChannelName {
			return nil, fmt.Errorf("recharge storefront channel %q name is too long", channel.ID)
		}

		channel.URL = strings.TrimSpace(channel.URL)
		if len(channel.URL) == 0 || len(channel.URL) > maxRechargeStorefrontURLLength {
			return nil, fmt.Errorf("invalid recharge storefront channel %q URL", channel.ID)
		}
		parsed, err := validateRechargeStorefrontURL(channel.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid recharge storefront channel %q URL: %w", channel.ID, err)
		}
		channel.URL = parsed.String()
		urlKey := strings.ToLower(channel.URL)
		if _, exists := seenURLs[urlKey]; exists {
			return nil, fmt.Errorf("duplicate recharge storefront channel URL %q", channel.URL)
		}
		seenURLs[urlKey] = struct{}{}
		normalized = append(normalized, channel)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		left := normalized[i]
		right := normalized[j]
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.ID < right.ID
	})
	for index := range normalized {
		normalized[index].SortOrder = index + 1
	}
	return normalized, nil
}

func parseRechargeStorefrontChannels(raw, legacyPrimary, legacyBackup string) []RechargeStorefrontChannel {
	var channels []RechargeStorefrontChannel
	if strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "null" {
		if err := json.Unmarshal([]byte(raw), &channels); err == nil {
			if normalized, normalizeErr := NormalizeRechargeStorefrontChannels(channels); normalizeErr == nil {
				return normalized
			}
		}
	}

	legacy := make([]RechargeStorefrontChannel, 0, 2)
	primary := strings.TrimSpace(legacyPrimary)
	if primary == "" {
		primary = defaultRechargeStorefrontURL
	}
	backup := strings.TrimSpace(legacyBackup)
	if backup == "" {
		backup = defaultRechargeStorefrontBackup
	}
	legacy = append(legacy,
		RechargeStorefrontChannel{ID: "backup-1", Name: "备用一", URL: primary, Enabled: true, SortOrder: 1},
		RechargeStorefrontChannel{ID: "backup-2", Name: "备用二", URL: backup, Enabled: true, SortOrder: 2},
	)
	normalized, err := NormalizeRechargeStorefrontChannels(legacy)
	if err != nil {
		return nil
	}
	return normalized
}

func enabledRechargeStorefrontChannels(channels []RechargeStorefrontChannel, enabled bool) []RechargeStorefrontChannel {
	if !enabled {
		return []RechargeStorefrontChannel{}
	}
	result := make([]RechargeStorefrontChannel, 0, len(channels))
	for _, channel := range channels {
		if channel.Enabled {
			result = append(result, channel)
		}
	}
	return result
}

func isSafeRechargeStorefrontID(value string) bool {
	for index, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || (index > 0 && (char == '-' || char == '_')) {
			continue
		}
		return false
	}
	return value != ""
}

func validateRechargeStorefrontURL(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("URL must be a complete https URL without credentials")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || host == "localhost" || !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".lan") || strings.HasSuffix(host, ".home.arpa") {
		return nil, fmt.Errorf("local hostnames are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return nil, fmt.Errorf("private or local IP addresses are not allowed")
	}
	parsed.Scheme = "https"
	return parsed, nil
}
