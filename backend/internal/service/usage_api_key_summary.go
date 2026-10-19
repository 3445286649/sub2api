package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type apiKeySummaryRepository interface {
	GetUserAPIKeySummary(context.Context, usagestats.APIKeySummaryQuery) (*usagestats.APIKeyUsageSummary, error)
	OwnsHistoricalAPIKey(context.Context, int64, int64) (bool, error)
}

func (s *UsageService) GetUserAPIKeySummary(ctx context.Context, q usagestats.APIKeySummaryQuery) (*usagestats.APIKeyUsageSummary, error) {
	r, ok := s.usageRepo.(apiKeySummaryRepository)
	if !ok {
		return nil, fmt.Errorf("api key summary repository unavailable")
	}
	return r.GetUserAPIKeySummary(ctx, q)
}

func (s *UsageService) OwnsHistoricalAPIKey(ctx context.Context, userID, keyID int64) (bool, error) {
	r, ok := s.usageRepo.(apiKeySummaryRepository)
	if !ok {
		return false, nil
	}
	return r.OwnsHistoricalAPIKey(ctx, userID, keyID)
}
