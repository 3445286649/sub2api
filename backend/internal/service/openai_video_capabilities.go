package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const videoModelCapabilitiesKey = "video_model_capabilities"

type videoRequestCapabilityKey struct{}
type videoRequestCapabilityCheck struct {
	info GrokMediaRequestInfo
	mu   sync.Mutex
	err  error
}

// WithOpenAIVideoRequest filters both scheduler engines before ranking/acquiring slots.
func WithOpenAIVideoRequest(ctx context.Context, info GrokMediaRequestInfo) context.Context {
	return context.WithValue(ctx, videoRequestCapabilityKey{}, &videoRequestCapabilityCheck{info: info})
}

func OpenAIVideoCapabilityError(ctx context.Context) error {
	check, _ := ctx.Value(videoRequestCapabilityKey{}).(*videoRequestCapabilityCheck)
	if check == nil {
		return nil
	}
	check.mu.Lock()
	defer check.mu.Unlock()
	return check.err
}

func accountSupportsVideoRequest(ctx context.Context, account *Account, model string, capability OpenAIEndpointCapability) bool {
	check, _ := ctx.Value(videoRequestCapabilityKey{}).(*videoRequestCapabilityCheck)
	if check == nil || capability != OpenAIEndpointCapabilityVideoTasks {
		return true
	}
	info := check.info
	info.Model = model
	if err := account.ValidateOpenAIVideoRequest(info); err != nil {
		check.mu.Lock()
		check.err = err
		check.mu.Unlock()
		return false
	}
	return true
}

// VideoModelCapability describes one exact upstream model; * supplies account defaults.
type VideoModelCapability struct {
	Model              string   `json:"model"`
	MaxReferenceImages int      `json:"max_reference_images"`
	MaxReferenceVideos int      `json:"max_reference_videos"`
	MaxReferenceAudios int      `json:"max_reference_audios"`
	SupportsFrames     bool     `json:"supports_frames"`
	Resolutions        []string `json:"resolutions"`
	MinDuration        int      `json:"min_duration"`
	MaxDuration        int      `json:"max_duration"`
}

func ParseVideoModelCapabilities(raw any) ([]VideoModelCapability, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid video model capabilities")
	}
	var items []VideoModelCapability
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > 64*1024 || decoder.Decode(&items) != nil || len(items) > 100 {
		return nil, fmt.Errorf("video model capabilities must be an array with at most 100 entries")
	}
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		item.Model = strings.TrimSpace(item.Model)
		if item.Model == "" || len(item.Model) > 256 || seen[item.Model] {
			return nil, fmt.Errorf("video capability model must be non-empty and unique")
		}
		seen[item.Model] = true
		if item.MaxReferenceImages < 0 || item.MaxReferenceImages > 30 || item.MaxReferenceVideos < 0 || item.MaxReferenceVideos > 10 || item.MaxReferenceAudios < 0 || item.MaxReferenceAudios > 10 {
			return nil, fmt.Errorf("video reference limits must be 0-30 images and 0-10 videos/audios")
		}
		if item.MinDuration < 0 || item.MaxDuration < 0 || item.MaxDuration > 600 || (item.MaxDuration > 0 && item.MinDuration > item.MaxDuration) || item.MinDuration > 600 {
			return nil, fmt.Errorf("video duration limits must be ordered within 0-600 seconds")
		}
		if len(item.Resolutions) == 0 || len(item.Resolutions) > 4 {
			return nil, fmt.Errorf("video resolutions are required")
		}
		for j, resolution := range item.Resolutions {
			normalized, ok := LookupOpenAIVideoResolution(resolution)
			if !ok {
				return nil, fmt.Errorf("video resolutions must be 480p, 720p, 1080p or 4k")
			}
			item.Resolutions[j] = normalized
		}
	}
	return items, nil
}

func ValidateVideoModelCredentials(credentials map[string]any) error {
	_, err := ParseVideoModelCapabilities(credentials[videoModelCapabilitiesKey])
	return err
}

// OpenAIVideoCapabilities defaults to the existing nine-image contract.
func (a *Account) OpenAIVideoCapabilities(model string) (VideoModelCapability, error) {
	capability := VideoModelCapability{MaxReferenceImages: 9, MaxReferenceVideos: 10, MaxReferenceAudios: 10, SupportsFrames: true, Resolutions: []string{"480p", "720p", "1080p"}}
	items, err := ParseVideoModelCapabilities(a.Credentials[videoModelCapabilitiesKey])
	if err != nil {
		return capability, err
	}
	model = a.GetMappedModel(model)
	for _, item := range items {
		if item.Model == model {
			return item, nil
		}
		if item.Model == "*" {
			capability = item
		}
	}
	return capability, nil
}

func (a *Account) ValidateOpenAIVideoRequest(info GrokMediaRequestInfo) error {
	capability, err := a.OpenAIVideoCapabilities(info.Model)
	if err != nil {
		return fmt.Errorf("video model capability configuration is invalid")
	}
	if info.ReferenceImageCount > capability.MaxReferenceImages || info.ReferenceVideoCount > capability.MaxReferenceVideos || info.ReferenceAudioCount > capability.MaxReferenceAudios {
		return fmt.Errorf("model supports at most %d reference images, %d reference videos and %d reference audios", capability.MaxReferenceImages, capability.MaxReferenceVideos, capability.MaxReferenceAudios)
	}
	if info.HasVideoFrames && !capability.SupportsFrames {
		return fmt.Errorf("model does not support first/last frames")
	}
	if info.DurationSeconds == 0 && (capability.MinDuration > 0 || capability.MaxDuration > 0) {
		return fmt.Errorf("duration is required for this model's configured limits")
	}
	for _, resolution := range capability.Resolutions {
		if resolution == info.Resolution {
			if info.DurationSeconds > 0 && (info.DurationSeconds < capability.MinDuration || (capability.MaxDuration > 0 && info.DurationSeconds > capability.MaxDuration)) {
				return fmt.Errorf("duration is outside the configured model range")
			}
			return nil
		}
	}
	return fmt.Errorf("model does not support resolution %s", info.Resolution)
}

// Keep OpenAI task 4K separate from Grok's three-resolution pricing contract.
func LookupOpenAIVideoResolution(value string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(value), "4k") || strings.TrimSpace(value) == "2160p" {
		return "4k", true
	}
	return LookupVideoBillingResolution(value)
}
