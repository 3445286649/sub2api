package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const SettingKeyHeaderButtons = "header_buttons"

type HeaderButton struct {
	Action  string `json:"action"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

func normalizeHeaderButtons(buttons []HeaderButton) ([]HeaderButton, error) {
	if len(buttons) > 12 {
		return nil, fmt.Errorf("at most 12 header buttons are allowed")
	}
	result := make([]HeaderButton, 0, len(buttons))
	seen := map[string]bool{}
	externalCount := 0
	for _, button := range buttons {
		button.Name = strings.TrimSpace(button.Name)
		button.URL = strings.TrimSpace(button.URL)
		if button.Name == "" || utf8.RuneCountInString(button.Name) > 32 {
			return nil, fmt.Errorf("header button name must contain 1 to 32 characters")
		}
		switch button.Action {
		case "external":
			externalCount++
			if externalCount > 6 || button.URL == "" {
				return nil, fmt.Errorf("at most 6 custom buttons are allowed and each requires a URL")
			}
		case "recharge", "rebate", "help", "radar", "support", "pixmo":
			if seen[button.Action] {
				return nil, fmt.Errorf("duplicate header button action: %s", button.Action)
			}
			seen[button.Action] = true
		default:
			return nil, fmt.Errorf("unsupported header button action")
		}
		if button.URL != "" {
			u, err := url.Parse(button.URL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || len(button.URL) > 2048 || strings.ContainsAny(button.URL, "\r\n\t\\") {
				return nil, fmt.Errorf("header button URL must be an absolute HTTP(S) URL without credentials")
			}
		}
		result = append(result, button)
	}
	return result, nil
}

func parseHeaderButtons(raw string) []HeaderButton {
	var buttons []HeaderButton
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &buttons) != nil || buttons == nil {
		return nil
	}
	result, err := normalizeHeaderButtons(buttons)
	if err != nil {
		return nil
	}
	return result
}
