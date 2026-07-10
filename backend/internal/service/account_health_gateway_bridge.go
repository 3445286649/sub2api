package service

import "context"

func (s *GatewayService) SetAccountHealthService(healthSvc *AccountHealthService) {
	if s != nil {
		s.accountHealthService = healthSvc
	}
}

func (s *GatewayService) RecordAccountHealthFailure(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 || accountHealthContextCanceled(ctx) || !shouldRecordAccountHealthFailure(failoverErr) {
		return
	}
	_ = s.accountHealthService.RecordFailure(ctx, accountID, accountHealthFailureCategory(failoverErr.StatusCode), string(failoverErr.ResponseBody))
}

func (s *GatewayService) RecordAccountHealthForwardError(ctx context.Context, accountID int64, err error) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 || accountHealthContextCanceled(ctx) || !shouldRecordAccountHealthForwardError(err) {
		return
	}
	_ = s.accountHealthService.RecordFailure(ctx, accountID, accountHealthForwardErrorCategory(err), err.Error())
}

func (s *GatewayService) RecordAccountHealthSuccess(ctx context.Context, accountID int64, latencyMs int64) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 {
		return
	}
	_ = s.accountHealthService.RecordSuccess(ctx, accountID, latencyMs)
}

func (s *GatewayService) loadAccountHealthSummaries(ctx context.Context, accounts []Account) map[int64]*AccountHealthSummary {
	if s == nil || s.accountHealthService == nil || len(accounts) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	summaries, err := s.accountHealthService.ListByAccountIDs(ctx, ids)
	if err != nil {
		return nil
	}
	return summaries
}

func (s *GatewayService) filterAccountsByHealthSchedulable(ctx context.Context, accounts []Account) []Account {
	if s == nil || s.accountHealthService == nil || len(accounts) == 0 {
		return accounts
	}
	health := s.loadAccountHealthSummaries(ctx, accounts)
	if len(health) == 0 {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if !accountHealthSummarySchedulable(health[account.ID]) {
			continue
		}
		filtered = append(filtered, account)
	}
	return filtered
}

func (s *OpenAIGatewayService) SetAccountHealthService(healthSvc *AccountHealthService) {
	if s != nil {
		s.accountHealthService = healthSvc
	}
}

func (s *OpenAIGatewayService) RecordAccountHealthFailure(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 || accountHealthContextCanceled(ctx) || !shouldRecordAccountHealthFailure(failoverErr) {
		return
	}
	_ = s.accountHealthService.RecordFailure(ctx, accountID, accountHealthFailureCategory(failoverErr.StatusCode), string(failoverErr.ResponseBody))
}

func (s *OpenAIGatewayService) RecordAccountHealthForwardError(ctx context.Context, accountID int64, err error) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 || accountHealthContextCanceled(ctx) || !shouldRecordAccountHealthForwardError(err) {
		return
	}
	_ = s.accountHealthService.RecordFailure(ctx, accountID, accountHealthForwardErrorCategory(err), err.Error())
}

func (s *OpenAIGatewayService) RecordAccountHealthSuccess(ctx context.Context, accountID int64, latencyMs int64) {
	if s == nil || s.accountHealthService == nil || accountID <= 0 {
		return
	}
	_ = s.accountHealthService.RecordSuccess(ctx, accountID, latencyMs)
}

func (s *OpenAIGatewayService) loadAccountHealthSummaries(ctx context.Context, accounts []Account) map[int64]*AccountHealthSummary {
	if s == nil || s.accountHealthService == nil || len(accounts) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	summaries, err := s.accountHealthService.ListByAccountIDs(ctx, ids)
	if err != nil {
		return nil
	}
	return summaries
}

func (s *OpenAIGatewayService) filterAccountsByHealthSchedulable(ctx context.Context, accounts []Account) []Account {
	if s == nil || s.accountHealthService == nil || len(accounts) == 0 {
		return accounts
	}
	health := s.loadAccountHealthSummaries(ctx, accounts)
	if len(health) == 0 {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if !accountHealthSummarySchedulable(health[account.ID]) {
			continue
		}
		filtered = append(filtered, account)
	}
	return filtered
}

func (s *OpenAIGatewayService) isAccountHealthSchedulable(ctx context.Context, accountID int64) bool {
	if s == nil || s.accountHealthService == nil || accountID <= 0 {
		return true
	}
	health, err := s.accountHealthService.ListByAccountIDs(ctx, []int64{accountID})
	if err != nil {
		return true
	}
	return accountHealthSummarySchedulable(health[accountID])
}

func accountHealthSummarySchedulable(summary *AccountHealthSummary) bool {
	return summary == nil || (summary.Status != AccountHealthStatusIsolated && summary.Status != AccountHealthStatusRecovering)
}
