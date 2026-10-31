package service

import (
	"context"
	"encoding/json"
)

// VideoCapabilities only discloses profiles reachable by the caller's bound group.
// Profiles remain separate so the client never combines incompatible account limits.
func (s *GatewayService) VideoCapabilities(ctx context.Context, group *Group, model string) ([]VideoModelCapability, error) {
	profiles := []VideoModelCapability{}
	if group == nil || model == "" {
		return profiles, nil
	}
	ctx = s.withGroupContext(ctx, group)
	platform, forced, err := s.resolvePlatform(ctx, &group.ID, group, model)
	if err != nil || platform != PlatformOpenAI {
		return profiles, err
	}
	if group.ModelAllowlistEnabled() && !group.ModelAllowlist.Allows(model) {
		return profiles, nil
	}
	if group.Platform == PlatformComposite {
		decision, ok, err := s.resolveCompositeRouteDecision(ctx, group, model, CompositeRouteEndpointAny)
		if err != nil || !ok {
			return profiles, err
		}
		model = decision.UpstreamModel
		ctx = WithCompositeRouteDecision(ctx, decision)
	}
	if s.checkChannelPricingRestriction(ctx, &group.ID, model) {
		return profiles, nil
	}
	accounts, _, err := s.listSchedulableAccounts(ctx, &group.ID, platform, forced)
	if err != nil {
		return profiles, err
	}
	seen := map[string]bool{}
	for i := range accounts {
		a := &accounts[i]
		if !a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideoTasks) || !s.isAccountSchedulableForSelection(a) ||
			(group.RequirePrivacySet && !a.IsPrivacySet()) || !s.isModelSupportedByAccountWithContext(ctx, a, model) ||
			s.isUpstreamModelRestrictedByChannel(ctx, group.ID, a, model) {
			continue
		}
		profile, err := a.OpenAIVideoCapabilities(model)
		if err != nil {
			continue
		}
		profile.Model = "" // Do not disclose upstream aliases or account identifiers.
		encoded, _ := json.Marshal(profile)
		if !seen[string(encoded)] {
			seen[string(encoded)] = true
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}
