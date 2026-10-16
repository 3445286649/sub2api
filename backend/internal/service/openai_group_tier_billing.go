package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// ServiceTierDetails is an immutable receipt of this request's tier decision.
// Nil on historical rows: never infer their customer tier from today's group.
type ServiceTierDetails struct {
	GroupTier     string                    `json:"group_tier,omitempty"`
	RequestedTier string                    `json:"requested_tier,omitempty"`
	UpstreamTier  string                    `json:"upstream_tier,omitempty"`
	BillingTier   string                    `json:"billing_tier"`
	Source        string                    `json:"source"`
	CustomerCosts *ServiceTierCustomerCosts `json:"customer_costs,omitempty"`
}

type openAIGroupTierSnapshot struct {
	GroupID  int64
	Tier     string
	Source   string
	FreeFast bool
}

type openAIGroupTierSnapshotKey struct{}

func openAIGroupRequestedModel(ctx context.Context, fallback string) string {
	if model, _ := ctx.Value(ctxkey.RequestedPublicModel).(string); model != "" {
		return model
	}
	if model, _ := ctx.Value(ctxkey.Model).(string); model != "" {
		return model
	}
	return fallback
}

func freezeOpenAIGroupTier(ctx context.Context, account *Account, model string) (context.Context, *openAIGroupTierSnapshot) {
	if ctx == nil || account == nil || !account.IsOpenAI() {
		return ctx, nil
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	if !IsGroupContextValid(group) || !groupSupportsOpenAIFast(group.Platform) {
		return ctx, nil
	}
	// Each forward attempt / WS turn gets its own scalar snapshot, including
	// the no-override case. Never retain the mutable per-model map for billing.
	tier := openAIGroupForcedTier(ctx, account, model)
	snapshot := &openAIGroupTierSnapshot{GroupID: group.ID, Tier: tier, FreeFast: group.FreeOpenAIFast, Source: "existing_policy"}
	if tier != "" {
		snapshot.Source = "group_default"
		if group.OpenAIModelTiers[openAIGroupRequestedModel(ctx, model)] == tier {
			snapshot.Source = "group_model"
		}
	}
	return context.WithValue(ctx, openAIGroupTierSnapshotKey{}, snapshot), snapshot
}

func stampOpenAIGroupTier(result *OpenAIForwardResult, snapshot *openAIGroupTierSnapshot) {
	if result != nil && result.groupTierSnapshot == nil {
		result.groupTierSnapshot = snapshot
	}
}

func customerServiceTier(tier string) string {
	switch normalizeBillingServiceTier(tier) {
	case "priority", "fast":
		return "priority"
	case "ultrafast":
		return "ultrafast"
	case "flex":
		return "flex"
	default:
		return "standard"
	}
}

func resolveOpenAIRecordServiceTier(apiKey *APIKey, account *Account, result *OpenAIForwardResult, requestedModel string) (*ServiceTierDetails, bool, error) {
	snapshot := result.groupTierSnapshot
	if snapshot == nil && apiKey != nil && apiKey.Group != nil {
		// Non-forwarding internal callers and legacy fixtures use their supplied
		// authenticated group. Normal HTTP/WS forwarding freezes before sending.
		ctx := context.WithValue(context.Background(), ctxkey.Group, apiKey.Group)
		if requestedModel == "" {
			requestedModel = result.Model
		}
		_, snapshot = freezeOpenAIGroupTier(ctx, account, requestedModel)
	}
	if snapshot != nil && (apiKey == nil || apiKey.GroupID == nil || apiKey.Group == nil || apiKey.Group.ID != snapshot.GroupID || *apiKey.GroupID != snapshot.GroupID) {
		return nil, false, fmt.Errorf("OpenAI billing group does not match request tier snapshot")
	}
	details := &ServiceTierDetails{RequestedTier: optionalStringValue(result.ServiceTier), UpstreamTier: result.UpstreamResponseServiceTier, Source: "existing_policy"}
	if snapshot != nil && snapshot.Tier != "" {
		details.GroupTier, details.Source = snapshot.Tier, snapshot.Source
		tier := snapshot.Tier
		result.ServiceTier = &tier
	} else {
		logServiceTierBillingDowngrade("service.openai_gateway", account, result.RequestID, ApplyOpenAIServiceTierBillingResolution(account, result))
	}
	details.BillingTier = customerServiceTier(optionalStringValue(result.ServiceTier))
	free := groupBillsOpenAIFastAtStandard(apiKey, account, optionalStringValue(result.ServiceTier))
	if snapshot != nil {
		free = snapshot.FreeFast && (details.BillingTier == "priority" || details.BillingTier == "ultrafast")
	}
	return details, free, nil
}

func openAIGroupTierPolicyConflict(ctx context.Context, account *Account, model, action string) string {
	tier := openAIGroupForcedTier(ctx, account, model)
	if tier != "" && (action == BetaPolicyActionFilter || (action == OpenAIFastPolicyActionForcePriority && tier != OpenAIFastTierPriority)) {
		return fmt.Sprintf("Group service_tier=%s conflicts with the OpenAI service tier policy (%s) for model %s; update the group or policy before retrying", tier, action, strings.TrimSpace(model))
	}
	return ""
}

func openAIWSTierPolicyContext(ctx context.Context, hooks *OpenAIWSIngressHooks, turn int, model string) context.Context {
	if hooks != nil && hooks.TierPolicyGroup != nil {
		group := hooks.TierPolicyGroup(turn)
		current, _ := ctx.Value(ctxkey.Group).(*Group)
		if IsGroupContextValid(group) && current != nil && group.ID == current.ID {
			ctx = context.WithValue(ctx, ctxkey.Group, group)
		}
	}
	return context.WithValue(ctx, ctxkey.RequestedPublicModel, model)
}

// Only populated when the customer's price basis differs from the cost basis.
type ServiceTierCustomerCosts struct {
	InputCost         float64 `json:"input_cost"`
	OutputCost        float64 `json:"output_cost"`
	CacheCreationCost float64 `json:"cache_creation_cost"`
	CacheReadCost     float64 `json:"cache_read_cost"`
	TotalCost         float64 `json:"total_cost"`
}
