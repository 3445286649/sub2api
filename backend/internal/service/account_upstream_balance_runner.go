package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const accountUpstreamBalanceRunnerPageSize = 100

type AccountUpstreamBalanceRunner struct {
	svc  *AccountUpstreamBalanceService
	cron *cron.Cron
}

func NewAccountUpstreamBalanceRunner(svc *AccountUpstreamBalanceService) *AccountUpstreamBalanceRunner {
	return &AccountUpstreamBalanceRunner{svc: svc}
}

func ProvideAccountUpstreamBalanceRunner(svc *AccountUpstreamBalanceService) *AccountUpstreamBalanceRunner {
	runner := NewAccountUpstreamBalanceRunner(svc)
	runner.Start()
	return runner
}

func (r *AccountUpstreamBalanceRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	c := cron.New(cron.WithParser(scheduledTestCronParser))
	if _, err := c.AddFunc("* * * * *", func() { r.runOnce() }); err != nil {
		logger.LegacyPrintf("service.account_upstream_balance_runner", "[AccountUpstreamBalanceRunner] not started: %v", err)
		return
	}
	r.cron = c
	r.cron.Start()
	logger.LegacyPrintf("service.account_upstream_balance_runner", "[AccountUpstreamBalanceRunner] started (tick=every minute)")
}

func (r *AccountUpstreamBalanceRunner) Stop() {
	if r == nil || r.cron == nil {
		return
	}
	ctx := r.cron.Stop()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		logger.LegacyPrintf("service.account_upstream_balance_runner", "[AccountUpstreamBalanceRunner] cron stop timed out")
	}
}

func (r *AccountUpstreamBalanceRunner) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	r.svc.RefreshDue(ctx, accountUpstreamBalanceRunnerPageSize)
}
