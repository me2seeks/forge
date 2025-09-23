package workpool

import (
	"context"
	"errors"
	"sync"

	"github.com/me2seeks/forge/pkg/safego"
)

var errWorkerPoolClosed = errors.New("worker pool 已关闭或取消")

type Job[T any] func(ctx context.Context) (T, error)

type Result[T any] struct {
	Value T
	Err   error
	Index int
}

type WorkerPool[T any] struct {
	jobQueue   chan Job[T]
	resultChan chan Result[T]
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func New[T any](ctx context.Context, workers, bufferSize int) *WorkerPool[T] {
	ctx, cancel := context.WithCancel(ctx)
	wp := &WorkerPool[T]{
		jobQueue:   make(chan Job[T], bufferSize),
		resultChan: make(chan Result[T], bufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}

	for range workers {
		safego.Go(ctx, wp.worker)
	}

	return wp
}

func (wp *WorkerPool[T]) worker() {
	for job := range wp.jobQueue {
		value, err := job(wp.ctx)
		result := Result[T]{Value: value, Err: err}
		select {
		case wp.resultChan <- result:
		case <-wp.ctx.Done():
			return
		}
		wp.wg.Done()
	}
}

func (wp *WorkerPool[T]) Submit(job Job[T]) error {
	select {
	case <-wp.ctx.Done():
		return errWorkerPoolClosed
	default:
		wp.wg.Add(1)
		select {
		case wp.jobQueue <- job:
			return nil
		case <-wp.ctx.Done():
			wp.wg.Done()
			return errWorkerPoolClosed
		}
	}
}

func (wp *WorkerPool[T]) Wait() {
	wp.wg.Wait()
}

func (wp *WorkerPool[T]) Close() {
	close(wp.jobQueue)
}

func (wp *WorkerPool[T]) Shutdown() {
	wp.cancel()
	wp.Close()
}

func (wp *WorkerPool[T]) Results() <-chan Result[T] {
	return wp.resultChan
}

func (wp *WorkerPool[T]) CollectAll() []Result[T] {
	wp.Wait()
	var results []Result[T]
	for result := range wp.resultChan {
		results = append(results, result)
	}
	return results
}
