package service

import (
	"context"
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// UserAccountRestrictions is scoped to a user, then group, then upstream account.
// It is never applied to a shared scheduler snapshot.
type UserAccountRestrictions map[int64][]int64

type UserAccountRestrictionRepository interface {
	GetByUserID(context.Context, int64) (UserAccountRestrictions, error)
	Replace(context.Context, int64, int64, []int64) error
}

var ErrInvalidAccountRestriction = infraerrors.BadRequest("INVALID_ACCOUNT_RESTRICTION", "Every blocked account must belong to the selected group")

type userAccountRestrictionsContextKey struct{}
type accountRestrictionGroupContextKey struct{}

// WithUserAccountRestrictions must only receive an authenticated user's rules.
func WithUserAccountRestrictions(ctx context.Context, restrictions UserAccountRestrictions) context.Context {
	return context.WithValue(ctx, userAccountRestrictionsContextKey{}, restrictions)
}

func withAccountRestrictionGroup(ctx context.Context, groupID *int64) context.Context {
	return context.WithValue(ctx, accountRestrictionGroupContextKey{}, derefGroupID(groupID))
}

func userAccountAllowed(ctx context.Context, account *Account) bool {
	if account == nil {
		return false
	}
	restrictions, _ := ctx.Value(userAccountRestrictionsContextKey{}).(UserAccountRestrictions)
	if len(restrictions) == 0 {
		return true
	}
	groupID, scoped := ctx.Value(accountRestrictionGroupContextKey{}).(int64)
	if !scoped {
		if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && group != nil {
			groupID = group.ID
		}
	}
	blocked := restrictions[groupID]
	return !slices.Contains(blocked, account.ID) &&
		(account.ParentAccountID == nil || !slices.Contains(blocked, *account.ParentAccountID))
}

func cloneUserAccountRestrictions(source UserAccountRestrictions) UserAccountRestrictions {
	if len(source) == 0 {
		return nil
	}
	result := make(UserAccountRestrictions, len(source))
	for groupID, ids := range source {
		result[groupID] = slices.Clone(ids)
	}
	return result
}
