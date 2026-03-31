package agnogo

import (
	"context"
	"sort"
	"sync"
	"time"
)

// WorkerPool processes messages through an agent using a fixed goroutine pool.
// Tasks are submitted to the pool and results are collected from a channel.
//
//	pool := agnogo.NewWorkerPool(agent, 4)
//	pool.Start(ctx)
//	pool.Submit(agnogo.WorkerTask{ID: "1", Message: "Hello"})
//	result := <-pool.Results()
//	pool.Stop()
type WorkerPool struct {
	agent   *Core
	workers int
	input   chan WorkerTask
	results chan WorkerResult
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// WorkerTask is a unit of work submitted to a WorkerPool.
type WorkerTask struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
}

// WorkerResult is the outcome of processing a WorkerTask.
type WorkerResult struct {
	TaskID   string        `json:"task_id"`
	Response *Response     `json:"response,omitempty"`
	Err      error         `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// NewWorkerPool creates a pool with buffered channels sized at workers * 2.
func NewWorkerPool(agent *Core, workers int) *WorkerPool {
	if workers < 1 {
		workers = 1
	}
	buf := workers * 2
	return &WorkerPool{
		agent:   agent,
		workers: workers,
		input:   make(chan WorkerTask, buf),
		results: make(chan WorkerResult, buf),
	}
}

// Submit sends a task to the pool's input channel. It blocks if the channel is full.
func (wp *WorkerPool) Submit(task WorkerTask) {
	wp.input <- task
}

// Results returns a receive-only channel of completed task results.
func (wp *WorkerPool) Results() <-chan WorkerResult {
	return wp.results
}

// Start launches worker goroutines that read from the input channel.
// Each worker creates a session (or uses RunWithStorage if the task specifies a
// SessionID and the agent has storage configured), calls Run, and sends the
// result to the results channel.
func (wp *WorkerPool) Start(ctx context.Context) {
	wp.ctx, wp.cancel = context.WithCancel(ctx)
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for task := range wp.input {
		start := time.Now()
		var (
			resp *Response
			err  error
		)

		if task.SessionID != "" && wp.agent.storage != nil {
			resp, err = wp.agent.RunWithStorage(wp.ctx, task.SessionID, task.Message)
		} else {
			session := NewSession(generateRunID())
			resp, err = wp.agent.Run(wp.ctx, session, task.Message)
		}

		wp.results <- WorkerResult{
			TaskID:   task.ID,
			Response: resp,
			Err:      err,
			Duration: time.Since(start),
		}
	}
}

// Stop closes the input channel, waits for all workers to finish processing,
// then closes the results channel.
func (wp *WorkerPool) Stop() {
	close(wp.input)
	wp.wg.Wait()
	if wp.cancel != nil {
		wp.cancel()
	}
	close(wp.results)
}

// Batch is a convenience function for one-shot parallel processing.
// It creates a pool, submits all tasks, collects every result, and returns
// them sorted by task ID to match the input order.
//
//	results := agnogo.Batch(ctx, agent, tasks, 4)
func Batch(ctx context.Context, agent *Core, tasks []WorkerTask, concurrency int) []WorkerResult {
	if len(tasks) == 0 {
		return nil
	}

	pool := NewWorkerPool(agent, concurrency)
	pool.Start(ctx)

	// Submit all tasks in a separate goroutine to avoid deadlock when the
	// number of tasks exceeds the channel buffer.
	go func() {
		for _, t := range tasks {
			pool.Submit(t)
		}
		close(pool.input)
	}()

	// Collect all results.
	results := make([]WorkerResult, 0, len(tasks))
	for i := 0; i < len(tasks); i++ {
		results = append(results, <-pool.results)
	}

	pool.wg.Wait()
	if pool.cancel != nil {
		pool.cancel()
	}

	// Sort by task ID for deterministic ordering.
	sort.Slice(results, func(i, j int) bool {
		return results[i].TaskID < results[j].TaskID
	})

	return results
}
