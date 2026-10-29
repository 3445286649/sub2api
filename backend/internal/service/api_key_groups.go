package service

import (
	"context"
	"errors"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const maxAPIKeyGroups = 20

type APIKeyGroupRouter interface {
	ProbeAPIKeyGroup(context.Context, *Group, string, string) (bool, error)
}

func (s *APIKeyService) SetGroupRouter(router APIKeyGroupRouter) { s.groupRouter = router }

func (k *APIKey) ConfiguredGroupIDs() []int64 {
	if k == nil {
		return nil
	}
	if len(k.GroupIDs) > 0 {
		return append([]int64(nil), k.GroupIDs...)
	}
	if k.GroupID != nil {
		return []int64{*k.GroupID}
	}
	return nil
}

func normalizeAPIKeyGroupIDs(primary *int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 && primary != nil {
		ids = []int64{*primary}
	}
	if len(ids) > maxAPIKeyGroups {
		return nil, infraerrors.BadRequest("INVALID_API_KEY_GROUPS", "At most 20 groups may be selected")
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, infraerrors.BadRequest("INVALID_API_KEY_GROUPS", "Group IDs must be positive")
		}
		if _, exists := seen[id]; exists {
			return nil, infraerrors.BadRequest("INVALID_API_KEY_GROUPS", "Group IDs must be unique")
		}
		seen[id] = struct{}{}
	}
	if primary != nil && len(ids) > 0 && ids[0] != *primary {
		return nil, infraerrors.BadRequest("INVALID_API_KEY_GROUPS", "group_id must match the first group_ids entry")
	}
	return append([]int64(nil), ids...), nil
}

func (s *APIKeyService) validateAPIKeyGroups(ctx context.Context, user *User, ids []int64) error {
	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
		if (len(ids) > 1 && !group.IsActive()) || !s.canUserBindGroup(ctx, user, group) {
			return ErrGroupNotAllowed
		}
	}
	return nil
}

func (s *APIKeyService) BillingGroups(ctx context.Context, key *APIKey) ([]*APIKey, error) {
	groups := make([]*APIKey, 0, len(key.ConfiguredGroupIDs()))
	for _, id := range key.ConfiguredGroupIDs() {
		group, err := s.groupRepo.GetByIDLite(ctx, id)
		if errors.Is(err, ErrGroupNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !s.canUserBindGroup(ctx, key.User, group) {
			continue
		}
		copyKey := *key
		copyKey.GroupID, copyKey.Group = &group.ID, group
		groups = append(groups, &copyKey)
	}
	return groups, nil
}

// PinAPIKeyGroup refreshes the selected group's pricing without changing the
// group chosen for an existing WebSocket connection.
func (s *APIKeyService) PinAPIKeyGroup(ctx context.Context, key *APIKey, id int64) (*APIKey, error) {
	if key == nil || key.User == nil {
		return nil, ErrGroupNotAllowed
	}
	bound := false
	for _, candidate := range key.ConfiguredGroupIDs() {
		if candidate == id {
			bound = true
			break
		}
	}
	if !bound {
		return nil, ErrGroupNotAllowed
	}
	group, err := s.groupRepo.GetByIDLite(ctx, id)
	if err != nil {
		return nil, err
	}
	if !group.IsActive() || !s.canUserBindGroup(ctx, key.User, group) {
		return nil, ErrGroupNotAllowed
	}
	selected := *key
	selected.GroupID, selected.Group = &group.ID, group
	return &selected, nil
}

// SelectRequestGroup picks one permitted, schedulable group before admission and billing.
func (s *APIKeyService) SelectRequestGroup(ctx context.Context, key *APIKey, models []string, kind string, google bool, checkBilling bool, subscriptions *SubscriptionService) (*APIKey, error) {
	if key == nil || len(key.ConfiguredGroupIDs()) < 2 {
		return key, nil
	}
	if s.groupRouter == nil || key.User == nil {
		return nil, infraerrors.ServiceUnavailable("MULTI_GROUP_ROUTING_UNAVAILABLE", "Multi-group routing is unavailable")
	}
	for _, id := range key.ConfiguredGroupIDs() {
		group, err := s.groupRepo.GetByIDLite(ctx, id)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue
			}
			return nil, err
		}
		if !group.IsActive() || !s.canUserBindGroup(ctx, key.User, group) {
			continue
		}
		if google && group.Platform != PlatformGemini && group.Platform != PlatformAntigravity && group.Platform != PlatformComposite {
			continue
		}
		if kind == "image" && !GroupAllowsImageGeneration(group) {
			continue
		}
		if checkBilling && !group.IsSubscriptionType() && key.User.Balance <= 0 {
			continue
		}
		if checkBilling && group.IsSubscriptionType() {
			if s.userSubRepo == nil {
				continue
			}
			sub, subErr := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, key.UserID, id)
			if subErr != nil {
				if errors.Is(subErr, ErrSubscriptionNotFound) || errors.Is(subErr, ErrSubscriptionExpired) {
					continue
				}
				return nil, subErr
			}
			if subscriptions != nil {
				copySub := *sub
				if _, subErr := subscriptions.ValidateAndCheckLimits(&copySub, group); subErr != nil {
					continue
				}
			}
		}
		allowed := true
		for _, model := range models {
			if group.ModelAllowlistEnabled() && !group.ModelAllowlist.Allows(model) {
				allowed = false
				break
			}
		}
		if !allowed {
			continue
		}
		probeModels := models
		if len(probeModels) == 0 {
			probeModels = []string{""}
		}
		available := true
		for _, model := range probeModels {
			var probeOK bool
			probeOK, err = s.groupRouter.ProbeAPIKeyGroup(ctx, group, model, kind)
			if err != nil {
				return nil, err
			}
			if !probeOK {
				available = false
				break
			}
		}
		if !available {
			continue
		}
		selected := *key
		selected.GroupID, selected.Group = &group.ID, group
		selected.User = &User{}
		*selected.User = *key.User
		selected.User.UserGroupRPMOverride = nil
		if s.userGroupRateRepo != nil {
			selected.User.UserGroupRPMOverride, err = s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, key.UserID, id)
			if err != nil {
				return nil, err
			}
		}
		return &selected, nil
	}
	return nil, infraerrors.ServiceUnavailable("NO_AVAILABLE_API_KEY_GROUP", "No selected group currently supports this request")
}
