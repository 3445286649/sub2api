package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type ModelRadarRunner struct {
	svc     *ModelRadarService
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
}

func NewModelRadarRunner(svc *ModelRadarService) *ModelRadarRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModelRadarRunner{svc: svc, ctx: ctx, cancel: cancel}
}

func (r *ModelRadarRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.wg.Add(1)
	r.mu.Unlock()
	go r.loop()
}

func (r *ModelRadarRunner) Stop() {
	if r == nil {
		return
	}
	r.cancel()
	r.wg.Wait()
}

func (r *ModelRadarRunner) loop() {
	defer r.wg.Done()
	timer := time.NewTimer(r.nextDelay())
	defer timer.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-timer.C:
			r.fire()
			timer.Reset(r.nextDelay())
		}
	}
}

func (r *ModelRadarRunner) fire() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cfg, err := r.svc.GetConfig(ctx)
	if err != nil || cfg == nil || !cfg.Enabled || !cfg.APIKeyConfigured {
		return
	}
	_, err = r.svc.Run(ctx, ModelRadarRunParams{TriggerType: ModelRadarTriggerScheduled, Now: time.Now()})
	if err != nil {
		slog.Warn("model_radar: scheduled run failed", "error", err)
	}
}

func (r *ModelRadarRunner) nextDelay() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg, err := r.svc.GetConfig(ctx)
	hour, minute := modelRadarDefaultRunHour, modelRadarDefaultRunMinute
	if err == nil && cfg != nil {
		hour, minute = cfg.RunHour, cfg.RunMinute
	}
	loc := modelRadarLocation()
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}

func ProvideModelRadarService(repo ModelRadarRepository, settingRepo SettingRepository, settingService *SettingService, encryptor SecretEncryptor, apiKeyService *APIKeyService) *ModelRadarService {
	return NewModelRadarService(repo, settingRepo, settingService, encryptor, apiKeyService)
}

func ProvideModelRadarRunner(svc *ModelRadarService) *ModelRadarRunner {
	r := NewModelRadarRunner(svc)
	r.Start()
	return r
}
