package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const accountHealthEventRetention = 30 * 24 * time.Hour

type AccountHealthEventCleanupRunner struct {
	healthSvc *AccountHealthService
	now       func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func NewAccountHealthEventCleanupRunner(healthSvc *AccountHealthService) *AccountHealthEventCleanupRunner {
	return &AccountHealthEventCleanupRunner{healthSvc: healthSvc, now: time.Now, stopCh: make(chan struct{})}
}

func ProvideAccountHealthEventCleanupRunner(healthSvc *AccountHealthService) *AccountHealthEventCleanupRunner {
	runner := NewAccountHealthEventCleanupRunner(healthSvc)
	runner.Start()
	return runner
}

func (r *AccountHealthEventCleanupRunner) Start() {
	if r == nil || r.healthSvc == nil {
		return
	}
	r.startOnce.Do(func() {
		logger.LegacyPrintf("service.account_health_event_cleanup", "[AccountHealthEventCleanup] started retention=%s", accountHealthEventRetention)
		go r.runLoop()
	})
}

func (r *AccountHealthEventCleanupRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
}

func (r *AccountHealthEventCleanupRunner) runLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	r.cleanupOnce()
	for {
		select {
		case <-ticker.C:
			r.cleanupOnce()
		case <-r.stopCh:
			return
		}
	}
}

func (r *AccountHealthEventCleanupRunner) cleanupOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deleted, err := r.healthSvc.CleanupEvents(ctx, r.now().Add(-accountHealthEventRetention))
	if err != nil {
		logger.LegacyPrintf("service.account_health_event_cleanup", "[AccountHealthEventCleanup] cleanup failed err=%v", err)
		return
	}
	if deleted > 0 {
		logger.LegacyPrintf("service.account_health_event_cleanup", "[AccountHealthEventCleanup] cleaned records count=%d", deleted)
	}
}
