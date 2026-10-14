package service

import (
	"regexp"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const accountImageModelsKey = "image_generation_models"

var imageModelIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// ValidateAccountImageModelsExtra validates the admin's explicit probe defaults.
// These labels do not grant group permissions or select an upstream protocol.
func ValidateAccountImageModelsExtra(extra map[string]any) error {
	raw, exists := extra[accountImageModelsKey]
	if !exists {
		return nil
	}
	var values []any
	switch models := raw.(type) {
	case []any:
		values = models
	case []string:
		for _, model := range models {
			values = append(values, model)
		}
	default:
		return infraerrors.BadRequest("INVALID_IMAGE_MODELS", "image_generation_models must be an array of model IDs")
	}
	if len(values) > 50 {
		return infraerrors.BadRequest("INVALID_IMAGE_MODELS", "image_generation_models must contain at most 50 model IDs")
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		model, ok := value.(string)
		model = strings.TrimSpace(model)
		if !ok || !imageModelIDPattern.MatchString(model) {
			return infraerrors.BadRequest("INVALID_IMAGE_MODELS", "invalid image generation model ID")
		}
		if !seen[model] {
			normalized = append(normalized, model)
			seen[model] = true
		}
	}
	extra[accountImageModelsKey] = normalized
	return nil
}

func (a *Account) hasImageModelLabel(model string) bool {
	switch models := a.Extra[accountImageModelsKey].(type) {
	case []string:
		for _, id := range models {
			if id == model {
				return true
			}
		}
	case []any:
		for _, id := range models {
			if text, ok := id.(string); ok && text == model {
				return true
			}
		}
	}
	return false
}
