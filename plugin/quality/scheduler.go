package quality

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type executionTask struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	frozen  Frozen
	source  string
	authIDs []string
}

// notifyWorkersLocked wakes idle workers after admission or dependency release.
func (en *Engine) notifyWorkersLocked() {
	if en.queueChanged != nil {
		close(en.queueChanged)
	}
	en.queueChanged = make(chan struct{})
}

// claimRunLocked reserves every dependency together; a blocked job never owns a worker.
func (en *Engine) claimRunLocked(id string) *executionTask {
	r := en.runs[id]
	if r == nil || r.Status != "queued" || en.closed || en.fault != nil {
		return nil
	}
	authIDs := dependencies(r.Config.RunConfig)
	for _, authID := range authIDs {
		if _, busy := en.busyAuth[authID]; busy {
			return nil
		}
	}
	next := cloneRun(r)
	next.Status = "running"
	c := changes()
	c.runs[id] = next
	if err := en.commitLocked(c); err != nil {
		return nil
	}
	if en.busyAuth == nil {
		en.busyAuth = map[string]struct{}{}
	}
	for _, authID := range authIDs {
		en.busyAuth[authID] = struct{}{}
	}
	ctx, cancel := context.WithCancel(en.ctx)
	en.active[id] = cancel
	return &executionTask{id: id, ctx: ctx, cancel: cancel, frozen: cloneFrozen(r.Config), source: r.Source, authIDs: authIDs}
}

// nextRunnableLocked rotates unavailable jobs without polling or exceeding queue capacity.
func (en *Engine) nextRunnableLocked() *executionTask {
	for pending := len(en.queue); pending > 0; pending-- {
		id := <-en.queue
		if ids, ok := en.scopeBatches[id]; ok {
			if task := en.rotateBatchLocked(id, ids, true); task != nil {
				return task
			}
			continue
		}
		r := en.runs[id]
		if r == nil || r.Status != "queued" {
			continue
		}
		if task := en.claimRunLocked(id); task != nil {
			return task
		}
		en.queue <- id
	}
	return nil
}

// compactQueueLocked frees terminal entries while preserving the remaining FIFO order.
func (en *Engine) compactQueueLocked() {
	for pending := len(en.queue); pending > 0; pending-- {
		id := <-en.queue
		if ids, ok := en.scopeBatches[id]; ok {
			en.rotateBatchLocked(id, ids, false)
			continue
		}
		if r := en.runs[id]; r != nil && r.Status == "queued" {
			en.queue <- id
		}
	}
}

func (en *Engine) worker() {
	defer en.wg.Done()
	for {
		en.mu.Lock()
		if en.closed || en.fault != nil {
			en.mu.Unlock()
			return
		}
		task := en.nextRunnableLocked()
		if en.fault != nil {
			en.mu.Unlock()
			return
		}
		if en.queueChanged == nil {
			en.queueChanged = make(chan struct{})
		}
		changed := en.queueChanged
		en.mu.Unlock()
		if task != nil {
			en.executeTask(task)
			continue
		}
		select {
		case <-en.ctx.Done():
			return
		case <-changed:
		}
	}
}
func (en *Engine) background() {
	defer en.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastReconcile, lastRetention time.Time
	for {
		select {
		case <-en.ctx.Done():
			return
		case event := <-en.events:
			if e := en.store.Put("usage", event.ID, event); e != nil {
				en.dropped.Add(1)
			}
		case now := <-ticker.C:
			if en.Capabilities().QualityPause && now.Sub(lastReconcile) >= 30*time.Second {
				if err := en.Reconcile(en.ctx); err != nil {
					log.Debug("model-quality: reconciliation remains pending")
				}
				lastReconcile = now
			}
			if now.Sub(lastRetention) >= time.Hour {
				if err := en.Prune(now.UTC()); err != nil {
					log.WithError(err).Error("model-quality: retention failed")
				}
				lastRetention = now
			}
			en.Tick(now.UTC())
		}
	}
}
func (en *Engine) Tick(now time.Time) {
	en.mu.Lock()
	if en.closed || en.fault != nil || (!en.cfg.SchedulingEnabled && len(en.timers) == 0) {
		en.mu.Unlock()
		return
	}
	if en.Capabilities().QualityPause {
		targets, err := en.host.Targets(en.ctx)
		if err != nil {
			en.mu.Unlock()
			return
		}
		for _, t := range targets {
			if t.QualityPaused {
				if err = en.haltCredentialLocked(t.AuthID, "", "quality_paused"); err != nil {
					en.mu.Unlock()
					return
				}
			}
		}
	}
	due, err := en.takeDueRulesLocked(now)
	inventoryCtx := en.inventoryContextLocked(en.ctx)
	en.mu.Unlock()
	if err != nil {
		return
	}
	if len(due) == 0 {
		return
	}
	status := &providerInventoryStatus{Verified: true}
	read := sync.OnceValues(func() ([]Target, error) {
		return en.host.Targets(context.WithValue(inventoryCtx, providerInventoryStatusKey{}, status))
	})
	inventory := func(ctx context.Context) ([]Target, error) {
		targets, err := read()
		if coverage, ok := ctx.Value(providerInventoryStatusKey{}).(*providerInventoryStatus); ok {
			coverage.Verified = status.Verified
		}
		return targets, err
	}
	for _, scheduled := range due {
		if _, err = en.dispatchWithInventory(en.ctx, scheduled.id, scheduled.revision, true, inventory); err != nil {
			log.WithField("rule_id", scheduled.id).Debug("model-quality: scheduled run not admitted")
		}
	}
}

type scheduledRule struct {
	id       string
	revision uint64
}

func (en *Engine) takeDueRulesLocked(now time.Time) ([]scheduledRule, error) {
	due := []scheduledRule{}
	c := changes()
	for _, r := range en.rules {
		if en.unsupportedRule(r) {
			continue
		}
		if en.timerAllowed(r) && ruleScheduled(r) && !r.NextRun.IsZero() && !r.NextRun.After(now) {
			n := cloneRule(r)
			n.NextRun = nextRun(*r, now)
			c.rules[r.ID] = n
			if n.NextRun.IsZero() {
				c.audits = append(c.audits, en.audit("rule_schedule_invalid", r.ID, "", "", "invalid stored schedule; no run admitted"))
				continue
			}
			due = append(due, scheduledRule{id: r.ID, revision: r.Revision})
		}
	}
	if err := en.commitLocked(c); err != nil {
		return nil, err
	}
	return due, nil
}
func (en *Engine) Usage(event UsageEvent) {
	if en.ctx != nil && en.ctx.Err() != nil {
		en.dropped.Add(1)
		return
	}
	if len(event.ID) > 128 || len(event.Model) > 256 || len(event.Provider) > 128 {
		en.dropped.Add(1)
		return
	}
	if event.ID == "" {
		return
	}
	event.Source = trafficSource(event.Source)
	select {
	case en.events <- event:
	default:
		en.dropped.Add(1)
	}
}
