package task

import (
	"runtime"
	"sync"

	"github.com/panjf2000/ants/v2"
)

const (
	// DefaultPoolSize is the default size of the worker pool
	// Use number of CPU cores by default for CPU-bound tasks
	DefaultPoolSize = 0 // Will be set to runtime.NumCPU() if 0

	DefaultQueueSize  = 1024
	NonBlockingSubmit = false
)

type WorkerPool struct {
	pool      *ants.Pool
	wg        sync.WaitGroup
	queueSize int
	poolSize  int
}

func NewWorkerPool(poolSize int) (*WorkerPool, error) {
	if poolSize <= 0 {
		poolSize = runtime.NumCPU()
	}

	pool, err := ants.NewPool(poolSize, ants.WithNonblocking(NonBlockingSubmit))
	if err != nil {
		return nil, err
	}

	return &WorkerPool{
		pool:      pool,
		queueSize: DefaultQueueSize,
		poolSize:  poolSize,
	}, nil
}

func (wp *WorkerPool) Submit(task func()) error {
	wp.wg.Add(1)
	return wp.pool.Submit(func() {
		defer wp.wg.Done()
		task()
	})
}

func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool) Release() {
	wp.pool.Release()
}

func (wp *WorkerPool) Running() int {
	return wp.pool.Running()
}

func (wp *WorkerPool) Free() int {
	return wp.pool.Free()
}

func (wp *WorkerPool) Capacity() int {
	return wp.pool.Cap()
}

func (wp *WorkerPool) Tune(size int) {
	wp.pool.Tune(size)
	wp.poolSize = size
}
