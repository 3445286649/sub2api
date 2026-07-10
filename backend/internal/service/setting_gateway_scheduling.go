package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const gatewaySchedulingWeightsCacheTTL = 30 * time.Second

type cachedGatewaySchedulingWeights struct {
	weights GatewaySchedulingWeights
	expires time.Time
	ok      bool
}

// GatewaySchedulingConfigWithRuntimeWeights overlays DB-configured scheduler weights onto the static gateway config.
func (s *SettingService) GatewaySchedulingConfigWithRuntimeWeights(ctx context.Context, base config.GatewaySchedulingConfig) config.GatewaySchedulingConfig {
	if s == nil || s.settingRepo == nil {
		return base
	}
	now := time.Now()
	if cached, ok := s.gatewaySchedulingWeightsCache.Load().(*cachedGatewaySchedulingWeights); ok && cached != nil && cached.expires.After(now) {
		if cached.ok {
			return applyGatewaySchedulingWeights(base, cached.weights)
		}
		return base
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyGatewaySchedulingWeights)
	if err != nil || strings.TrimSpace(raw) == "" {
		s.gatewaySchedulingWeightsCache.Store(&cachedGatewaySchedulingWeights{expires: now.Add(gatewaySchedulingWeightsCacheTTL)})
		return base
	}
	var weights GatewaySchedulingWeights
	if err := json.Unmarshal([]byte(raw), &weights); err != nil || !validGatewaySchedulingWeights(weights) {
		s.gatewaySchedulingWeightsCache.Store(&cachedGatewaySchedulingWeights{expires: now.Add(gatewaySchedulingWeightsCacheTTL)})
		return base
	}
	s.gatewaySchedulingWeightsCache.Store(&cachedGatewaySchedulingWeights{weights: weights, expires: now.Add(gatewaySchedulingWeightsCacheTTL), ok: true})
	return applyGatewaySchedulingWeights(base, weights)
}

func applyGatewaySchedulingWeights(base config.GatewaySchedulingConfig, weights GatewaySchedulingWeights) config.GatewaySchedulingConfig {
	base.ScoreWeightHealth = weights.Health
	base.ScoreWeightLatency = weights.Latency
	base.ScoreWeightCost = weights.Cost
	base.ScoreWeightLoad = weights.Load
	return base
}
