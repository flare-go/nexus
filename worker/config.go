package worker

import "time"

// Config contains all configurations for the worker pool
type Config struct {
	MaxWorkers     int           // maximum number of workers in the pool
	ExpiryDuration time.Duration // how long to wait before killing idle workers
	PreAlloc       bool          // whether to allocate workers when pool is created
	MaxBlockTasks  int           // maximum number of tasks allowed to be blocked
	Nonblocking    bool          // whether to return error when pool is full
}

// DefaultConfig worker pool default config
// 100 worker pools max workers
// 10000 worker pool max block tasks
// 1 minute worker pool expiry duration
// true worker pool pre alloc
// false worker pool nonblocking
func DefaultConfig() Config {
	return Config{
		MaxWorkers:     100,
		MaxBlockTasks:  10000,
		ExpiryDuration: time.Minute,
		PreAlloc:       true,
		Nonblocking:    false,
	}
}
