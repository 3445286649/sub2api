package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type apiKeyBreakdownRepository interface {
	GetUserAPIKeyBreakdown(context.Context, usagestats.APIKeyBreakdownQuery) (*usagestats.APIKeyUsageBreakdown, error)
}

func (s *UsageService) GetUserAPIKeyBreakdown(ctx context.Context, q usagestats.APIKeyBreakdownQuery) (*usagestats.APIKeyUsageBreakdown, error) {
	r, ok := s.usageRepo.(apiKeyBreakdownRepository)
	if !ok {
		return nil, fmt.Errorf("api key breakdown repository unavailable")
	}
	return r.GetUserAPIKeyBreakdown(ctx, q)
}
