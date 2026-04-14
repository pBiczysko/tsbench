// Package bench orchestrates the benchmark process by distributing jobs across a pool of workers.
package bench

import (
	"context"
	"errors"
	"hash/fnv"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Bench is an orchestrator for the benchmark process.
type Bench struct {
	jobs       <-chan JobParams
	maxWorkers int
	repo       repository
	log        *slog.Logger
}

type repository interface {
	MeasureGetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error)
}

// New creates a new Bench to execute jobs against db with max concurrent workers.
func New(jobs <-chan JobParams, maxWorkers int, repo repository, log *slog.Logger) Bench {
	return Bench{
		jobs:       jobs,
		maxWorkers: maxWorkers,
		repo:       repo,
		log:        log,
	}
}

// Process runs the benchmark by creating a worker pool and processing jobs concurrently.
// It uses a hash of the hostname to route each job always to the same worker channel.
// Each worker sends result data to the res channel, which is aggregated into a Summary.
// Process returns the statistical summary for query execution.
func (b Bench) Process(ctx context.Context) Summary {
	workerJobs := make([]chan JobParams, b.maxWorkers)
	for i := range b.maxWorkers {
		// Buffer of 1 allows distributor to be one job ahead of each worker.
		// A larger buffer would reduce csv stream blocking but the optimal value
		// depends on workload.
		workerJobs[i] = make(chan JobParams, 1)
	}

	closeWorkerJobs := func() {
		for _, ch := range workerJobs {
			close(ch)
		}
	}

	res := make(chan result, b.maxWorkers)

	wg := sync.WaitGroup{}
	wg.Add(b.maxWorkers)

	for _, ch := range workerJobs {
		go b.runWorker(ctx, ch, &wg, res)
	}

	// Distribute jobs to specific worker channels based on hostname hash.
	go func() {
		defer closeWorkerJobs()

		for j := range b.jobs {
			idx := getChannelIndex(j.Hostname, b.maxWorkers)
			workerJobs[idx] <- j
		}
	}()

	go func() {
		wg.Wait()
		close(res)
	}()

	results := collectResults(res)

	return generateSummary(results)
}

func (b Bench) runWorker(ctx context.Context, in <-chan JobParams, wg *sync.WaitGroup, out chan<- result) {
	defer wg.Done()
	for j := range in {
		dur, err := b.repo.MeasureGetCPUUsage(ctx, j.Hostname, j.StartTime, j.EndTime)
		if err != nil {
			out <- result{failed: true}
			// Cancelled context will propagate to db.
			if !errors.Is(err, context.Canceled) {
				b.log.Error("error executing query", "error", err)
			}
			continue
		}
		out <- result{duration: dur, failed: false}
	}
}

func collectResults(results <-chan result) []result {
	out := make([]result, 0)
	for r := range results {
		out = append(out, r)
	}

	return out
}

func generateSummary(results []result) Summary {
	durations := make([]time.Duration, 0, len(results))
	var failedCount int
	var total time.Duration

	for _, r := range results {
		if r.failed {
			failedCount++
			continue
		}
		durations = append(durations, r.duration)
		total += r.duration
	}

	var min time.Duration
	var max time.Duration
	var median time.Duration
	var avg time.Duration

	if len(durations) > 0 {
		slices.Sort(durations)
		min = durations[0]
		max = durations[len(durations)-1]
		median = durations[len(durations)/2]
		avg = total / time.Duration(len(durations))
		if len(durations)%2 == 0 {
			median = (durations[(len(durations)/2)-1] + durations[len(durations)/2]) / 2
		}
	}

	return Summary{
		MinDuration:    min,
		MaxDuration:    max,
		AvgDuration:    avg,
		MedianDuration: median,
		TotalTime:      total,
		TotalCount:     len(durations) + failedCount,
		FailedCount:    failedCount,
	}
}

func getChannelIndex(hostname string, maxWorkers int) int {
	h := fnv.New32a()
	h.Write([]byte(hostname))

	return int(h.Sum32() % uint32(maxWorkers))
}
