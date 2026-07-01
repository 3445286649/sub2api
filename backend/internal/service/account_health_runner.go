package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const accountHealthProbeMaxWorkers = 5

type AccountHealthRunner struct {
	healthSvc      *AccountHealthService
	accountTestSvc *AccountTestService
	cron           *cron.Cron
	startOnce      sync.Once
	stopOnce       sync.Once
	runningMu      sync.Mutex
	running        map[int64]struct{}
}

func NewAccountHealthRunner(healthSvc *AccountHealthService, accountTestSvc *AccountTestService) *AccountHealthRunner {
	return &AccountHealthRunner{healthSvc: healthSvc, accountTestSvc: accountTestSvc, running: make(map[int64]struct{})}
}

func ProvideAccountHealthRunner(healthSvc *AccountHealthService, accountTestSvc *AccountTestService) *AccountHealthRunner {
	runner := NewAccountHealthRunner(healthSvc, accountTestSvc)
	runner.Start()
	return runner
}

func (r *AccountHealthRunner) Start() {
	if r == nil || r.healthSvc == nil || r.accountTestSvc == nil {
		return
	}
	r.startOnce.Do(func() {
		c := cron.New(cron.WithParser(scheduledTestCronParser))
		if _, err := c.AddFunc("* * * * *", func() { r.runDueProbes() }); err != nil {
			logger.LegacyPrintf("service.account_health_runner", "[AccountHealthRunner] not started: %v", err)
			return
		}
		r.cron = c
		r.cron.Start()
		logger.LegacyPrintf("service.account_health_runner", "[AccountHealthRunner] started (tick=every minute)")
	})
}

func (r *AccountHealthRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		if r.cron == nil {
			return
		}
		ctx := r.cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
			logger.LegacyPrintf("service.account_health_runner", "[AccountHealthRunner] cron stop timed out")
		}
	})
}

func (r *AccountHealthRunner) runDueProbes() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	states, err := r.healthSvc.ListDueForProbe(ctx, time.Now(), accountHealthDefaultProbePageSize)
	if err != nil {
		logger.LegacyPrintf("service.account_health_runner", "[AccountHealthRunner] ListDueForProbe error: %v", err)
		return
	}
	if len(states) == 0 {
		return
	}
	sem := make(chan struct{}, accountHealthProbeMaxWorkers)
	var wg sync.WaitGroup
	for _, state := range states {
		if state == nil || !r.tryClaim(state.AccountID) {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(accountID int64) {
			defer wg.Done()
			defer func() { <-sem }()
			defer r.release(accountID)
			r.probeOne(ctx, accountID)
		}(state.AccountID)
	}
	wg.Wait()
}

func (r *AccountHealthRunner) tryClaim(accountID int64) bool {
	r.runningMu.Lock()
	defer r.runningMu.Unlock()
	if r.running == nil {
		r.running = make(map[int64]struct{})
	}
	if _, ok := r.running[accountID]; ok {
		return false
	}
	r.running[accountID] = struct{}{}
	return true
}

func (r *AccountHealthRunner) release(accountID int64) {
	r.runningMu.Lock()
	defer r.runningMu.Unlock()
	delete(r.running, accountID)
}

func (r *AccountHealthRunner) probeOne(ctx context.Context, accountID int64) {
	result, err := r.accountTestSvc.RunTestBackground(ctx, accountID, r.healthSvc.HealthProbeModel(ctx, accountID))
	if err != nil {
		_ = r.healthSvc.RecordProbeFailure(ctx, accountID, "probe_error", err.Error())
		return
	}
	if result != nil && result.Status == "success" {
		_ = r.healthSvc.RecordProbeSuccess(ctx, accountID, result.LatencyMs)
		return
	}
	message := ""
	if result != nil {
		message = result.ErrorMessage
	}
	_ = r.healthSvc.RecordProbeFailure(ctx, accountID, accountHealthProbeFailureCategory(message), message)
}
