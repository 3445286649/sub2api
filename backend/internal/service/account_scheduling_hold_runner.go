package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type AccountSchedulingHoldExpiryRunner struct {
	service  *AccountSchedulingHoldService
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	start    sync.Once
	stop     sync.Once
}

func NewAccountSchedulingHoldExpiryRunner(service *AccountSchedulingHoldService, interval time.Duration) *AccountSchedulingHoldExpiryRunner {
	if interval <= 0 {
		interval = time.Minute
	}
	return &AccountSchedulingHoldExpiryRunner{
		service: service, interval: interval, stopCh: make(chan struct{}), doneCh: make(chan struct{}),
	}
}

func ProvideAccountSchedulingHoldExpiryRunner(service *AccountSchedulingHoldService) *AccountSchedulingHoldExpiryRunner {
	runner := NewAccountSchedulingHoldExpiryRunner(service, time.Minute)
	runner.Start()
	return runner
}

func (r *AccountSchedulingHoldExpiryRunner) Start() {
	if r == nil || r.service == nil {
		return
	}
	r.start.Do(func() {
		go func() {
			defer close(r.doneCh)
			ticker := time.NewTicker(r.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					r.runOnce()
				case <-r.stopCh:
					return
				}
			}
		}()
	})
}

func (r *AccountSchedulingHoldExpiryRunner) Stop() {
	if r == nil {
		return
	}
	r.stop.Do(func() { close(r.stopCh) })
	select {
	case <-r.doneCh:
	case <-time.After(3 * time.Second):
	}
}

func (r *AccountSchedulingHoldExpiryRunner) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ids, err := r.service.ExpireDue(ctx)
	if err != nil {
		logger.LegacyPrintf("service.account_scheduling_hold", "[SchedulingHold] expire pass failed err=%v", err)
		return
	}
	if len(ids) > 0 {
		logger.LegacyPrintf("service.account_scheduling_hold", "[SchedulingHold] expired leases count=%d", len(ids))
	}
}
