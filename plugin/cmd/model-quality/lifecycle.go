package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/cliproxyapi-plugins/model-quality/quality"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

type runtimeEngine interface {
	Close() error
	Handle(context.Context, pluginapi.ManagementRequest) pluginapi.ManagementResponse
	Usage(quality.UsageEvent)
}

type engineInstance struct {
	engine runtimeEngine
	ctx    context.Context
	cancel context.CancelFunc
	users  sync.WaitGroup
}

// Transitions serialize replacement and shutdown; leases keep each store alive
// until its already-admitted management and usage callbacks have returned.
type engineRuntime struct {
	transition sync.Mutex
	mu         sync.Mutex
	current    *engineInstance
}

func (r *engineRuntime) acquire() (*engineInstance, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return nil, nil, fmt.Errorf("unavailable: plugin is not running")
	}
	instance := r.current
	instance.users.Add(1)
	var once sync.Once
	return instance, func() { once.Do(instance.users.Done) }, nil
}

// stopCurrent runs only under transition, without holding mu while waiting.
func (r *engineRuntime) stopCurrent() error {
	r.mu.Lock()
	instance := r.current
	r.current = nil
	r.mu.Unlock()
	if instance == nil {
		return nil
	}
	instance.cancel()
	instance.users.Wait()
	return instance.engine.Close()
}

func (r *engineRuntime) replace(create func() (runtimeEngine, error)) error {
	r.transition.Lock()
	defer r.transition.Unlock()
	if err := r.stopCurrent(); err != nil {
		return err
	}
	engine, err := create()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.current = &engineInstance{engine: engine, ctx: ctx, cancel: cancel}
	r.mu.Unlock()
	return nil
}

func (r *engineRuntime) close() error {
	r.transition.Lock()
	defer r.transition.Unlock()
	return r.stopCurrent()
}
