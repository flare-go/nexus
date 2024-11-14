package worker

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"

	"go.uber.org/zap"
)

// Pool implements a worker pool based on ants pool
type Pool struct {
	pool    *ants.Pool
	logger  *zap.Logger
	metrics *Metrics
}

// Metrics tracks various statistics of the worker pool
type Metrics struct {
	CompletedTasks atomic.Int64 // number of successfully completed tasks
	FailedTasks    atomic.Int64 // number of failed tasks (including panics)
	RunningTasks   atomic.Int64 // number of tasks currently running
	WaitingTasks   atomic.Int64 // number of tasks waiting to be processed
}

// NewPool creates a new worker pool with a given configuration
// Returns error if pool creation fails
func NewPool(cfg Config, logger *zap.Logger) (*Pool, error) {
	// Validate configuration
	if cfg.MaxWorkers <= 0 {
		return nil, fmt.Errorf("max workers must be positive")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Configure ants pool options
	opts := ants.Options{
		ExpiryDuration:   cfg.ExpiryDuration,
		PreAlloc:         cfg.PreAlloc,
		MaxBlockingTasks: cfg.MaxBlockTasks,
		Nonblocking:      cfg.Nonblocking,
		PanicHandler: func(i interface{}) {
			logger.Error("worker panic recovered",
				zap.Any("error", i),
				zap.String("stack", string(debug.Stack())))
		},
	}

	// Create a new ants pool instance
	pool, err := ants.NewPool(cfg.MaxWorkers, ants.WithOptions(opts))
	if err != nil {
		return nil, fmt.Errorf("failed to create ants pool: %w", err)
	}

	return &Pool{
		pool:    pool,
		logger:  logger,
		metrics: &Metrics{},
	}, nil
}

// Submit submits a task to the worker pool
// Returns error if submission fails or context is canceled
func (p *Pool) Submit(ctx context.Context, task func() error) error {
	// Check context before submission
	select {
	case <-ctx.Done():
		p.metrics.FailedTasks.Add(1)
		return ctx.Err()
	default:
	}

	p.metrics.WaitingTasks.Add(1)

	// Wrap the task with metrics tracking
	wrappedTask := func() {
		defer func() {
			p.metrics.RunningTasks.Add(-1)
			p.metrics.WaitingTasks.Add(-1)
			if r := recover(); r != nil {
				p.metrics.FailedTasks.Add(1)
				p.logger.Error("worker recovered from panic",
					zap.Any("error", r),
					zap.String("stack", string(debug.Stack())))
				return
			}
		}()

		p.metrics.RunningTasks.Add(1)
		if err := task(); err != nil {
			p.metrics.FailedTasks.Add(1)
		}
		p.metrics.CompletedTasks.Add(1)
	}

	// Submit to ants pool
	err := p.pool.Submit(wrappedTask)
	if err != nil {
		p.metrics.FailedTasks.Add(1)
		p.metrics.WaitingTasks.Add(-1)
		return fmt.Errorf("failed to submit task: %w", err)
	}

	return nil
}

// Running returns the number of currently running workers
func (p *Pool) Running() int {
	return p.pool.Running()
}

// Cap returns the capacity of the pool
func (p *Pool) Cap() int {
	return p.pool.Cap()
}

// Free returns the number of available workers
func (p *Pool) Free() int {
	return p.pool.Free()
}

// GetMetrics returns current metrics of the pool
func (p *Pool) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"completed_tasks": p.metrics.CompletedTasks.Load(),
		"failed_tasks":    p.metrics.FailedTasks.Load(),
		"running_tasks":   p.metrics.RunningTasks.Load(),
		"waiting_tasks":   p.metrics.WaitingTasks.Load(),
		"capacity":        p.Cap(),
		"free_workers":    p.Free(),
	}
}

// Tune dynamically adjusts the size of the pool
// size: new size of the pool
func (p *Pool) Tune(size int) error {
	if size <= 0 {
		return fmt.Errorf("pool size must be positive")
	}
	p.pool.Tune(size)
	return nil
}

// Release releases all resources of the pool
// Should be called when the pool is no longer needed
func (p *Pool) Release() {
	p.pool.Release()
}

// GracefulShutdown waits for all tasks to complete and releases resources
// timeout: maximum time to wait for tasks to complete
func (p *Pool) GracefulShutdown(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		p.pool.Release()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timed out after %v", timeout)
	}
}
