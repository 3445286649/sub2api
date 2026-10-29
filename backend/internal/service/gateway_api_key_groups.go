package service

import "context"

// ProbeAPIKeyGroup reads scheduler eligibility without reserving slots or binding a session.
func (s *GatewayService) ProbeAPIKeyGroup(ctx context.Context, group *Group, model, kind string) (bool, error) {
	ctx = s.withGroupContext(ctx, group)
	platform, forced, err := s.resolvePlatform(ctx, &group.ID, group, model)
	if err != nil {
		return false, nil
	}
	if group.Platform == PlatformComposite && model != "" {
		decision, ok, err := s.resolveCompositeRouteDecision(ctx, group, model, CompositeRouteEndpointAny)
		if err != nil || !ok {
			return false, err
		}
		model = decision.UpstreamModel
		ctx = WithCompositeRouteDecision(ctx, decision)
	}
	if kind == "token" {
		ctx = s.withGatewayProfitControlGate(ctx, &group.ID)
	}
	if s.checkChannelPricingRestriction(ctx, &group.ID, model) {
		return false, nil
	}
	accounts, _, err := s.listSchedulableAccounts(ctx, &group.ID, platform, forced)
	if err != nil {
		return false, err
	}
	ctx = s.withWindowCostPrefetch(ctx, accounts)
	ctx = s.withRPMPrefetch(ctx, accounts)
	for i := range accounts {
		account := &accounts[i]
		if !s.isAccountSchedulableForSelection(account) || !s.isGatewayAccountProfitEligible(ctx, account) ||
			(group.RequirePrivacySet && !account.IsPrivacySet()) ||
			(model != "" && !s.isModelSupportedByAccountWithContext(ctx, account, model)) ||
			!s.isAccountSchedulableForModelSelection(ctx, account, model) ||
			!s.isAccountSchedulableForQuota(account) ||
			!s.isAccountSchedulableForWindowCost(ctx, account, false) ||
			!s.isAccountSchedulableForRPM(ctx, account, false) ||
			s.isUpstreamModelRestrictedByChannel(ctx, group.ID, account, model) {
			continue
		}
		return true, nil
	}
	return false, nil
}
