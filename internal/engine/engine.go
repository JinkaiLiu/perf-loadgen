package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JinkaiLiu/perf-loadgen/internal/config"
	"github.com/JinkaiLiu/perf-loadgen/internal/runner"
	"github.com/JinkaiLiu/perf-loadgen/internal/stats"
	"github.com/JinkaiLiu/perf-loadgen/pkg/types"
)

// Observer receives live request results and the final summary.
type Observer interface {
	Observe(types.RunResult)
	Finalize(types.Summary)
}

// RunReport is the detailed engine result used for distributed aggregation.
type RunReport struct {
	Summary  types.Summary
	Snapshot types.AggregateSnapshot
}

// Engine coordinates workers and aggregates results for a run.
type Engine struct {
	runner    runner.Runner
	observers []Observer
}

// New creates an engine with the provided runner implementation.
func New(r runner.Runner, observers ...Observer) *Engine {
	return &Engine{runner: r, observers: observers}
}

// Run executes a duration-bound load test with optional QPS limiting and ramp-up.
func (e *Engine) Run(ctx context.Context, cfg config.Config) (types.Summary, error) {
	report, err := e.RunDetailed(ctx, cfg)
	if err != nil {
		return types.Summary{}, err
	}
	return report.Summary, nil
}

// RunDetailed executes a run and returns both the final summary and mergeable snapshot.
func (e *Engine) RunDetailed(ctx context.Context, cfg config.Config) (RunReport, error) {
	startTime := time.Now()
	runCtx, cancel := context.WithCancel(ctx)
	if cfg.Duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.Duration)
	}
	defer cancel()

	results := make(chan types.RunResult, cfg.Concurrency*2)
	requestTemplate := types.RequestSpec{
		URL:     cfg.URL,
		Method:  cfg.Method,
		Headers: cfg.Headers,
		Timeout: cfg.Timeout,
	}
	payloadPicker := newPayloadPicker(cfg)
	var issued atomic.Int64

	var workers sync.WaitGroup
	pacer := newPacer(cfg.QPS)

	startWorker := func(delay time.Duration) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-runCtx.Done():
					return
				case <-timer.C:
				}
			}

			for {
				select {
				case <-runCtx.Done():
					return
				default:
				}

				if err := pacer.Wait(runCtx); err != nil {
					return
				}

				if cfg.Requests > 0 {
					next := issued.Add(1)
					if next > cfg.Requests {
						return
					}
				}

				requestSpec := requestTemplate
				requestSpec.Body = payloadPicker.Next()

				result, err := e.runner.Run(runCtx, requestSpec)
				if err != nil && result.ErrorCategory == types.ErrorCategoryNone {
					result.Success = false
					result.ErrorCategory = types.ErrorCategoryUnknown
					result.ErrorMessage = err.Error()
				}

				select {
				case results <- result:
				case <-runCtx.Done():
					return
				}

				// Prevent CPU spin on instant failures (e.g. connection refused).
				if !result.Success && result.Latency < time.Millisecond {
					select {
					case <-runCtx.Done():
						return
					case <-time.After(time.Millisecond):
					}
				}
			}
		}()
	}

	if cfg.RampUp <= 0 || cfg.Concurrency == 1 {
		for range cfg.Concurrency {
			startWorker(0)
		}
	} else {
		stepInterval := cfg.RampUp / time.Duration(cfg.Concurrency)
		if stepInterval <= 0 {
			stepInterval = time.Millisecond
		}
		for i := range cfg.Concurrency {
			startWorker(time.Duration(i) * stepInterval)
		}
	}

	go func() {
		workers.Wait()
		close(results)
	}()

	collector := stats.NewCollector()
	if cfg.ModelPricePer1K > 0 {
		collector.SetPricing(cfg.ModelPricePer1K)
	}
	for result := range results {
		collector.Add(result)
		for _, observer := range e.observers {
			observer.Observe(result)
		}
	}
	collector.SetWindow(startTime, time.Now())
	summary := collector.Summary()
	for _, observer := range e.observers {
		observer.Finalize(summary)
	}

	return RunReport{
		Summary:  summary,
		Snapshot: collector.Snapshot(),
	}, nil
}
