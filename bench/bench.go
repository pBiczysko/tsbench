// Package bench orchestrates the benchmark process by distributing jobs across a pool of workers.
package bench

import (
	"context"
	"hash/fnv"
	"slices"
	"sync"
	"time"
)

// Bench is an orchestrator for the benchmark process.
type Bench struct {
	jobs       <-chan JobParams
	maxWorkers int
	repo       repository
}

type repository interface {
	GetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error)
}

// New creates a new Bench to execute jobs against db with max concurrent workers.
func New(jobs <-chan JobParams, maxWorkers int, repo repository) Bench {
	return Bench{
		jobs:       jobs,
		maxWorkers: maxWorkers,
		repo:       repo,
	}
}

// Process runs the benchmark by creating a worker pool and processing jobs concurrently.
// It uses a hash of the hostname to route each job always to the same worker channel.
// Each worker sends result data to the res channel, which is aggregated into a Summary.
// Process returns the statistical summary for query execution.
func (b Bench) Process(ctx context.Context) Summary {
	workerJobs := make([]chan JobParams, b.maxWorkers)
	for i := range b.maxWorkers {
		workerJobs[i] = make(chan JobParams, 1)
	}

	res := make(chan result, b.maxWorkers)

	wg := sync.WaitGroup{}
	wg.Add(b.maxWorkers)

	for _, ch := range workerJobs {
		go b.runWorker(ctx, ch, &wg, res)
	}

	go func() {
		for j := range b.jobs {
			idx := getChannelIndex(j.Hostname, b.maxWorkers)
			workerJobs[idx] <- j
		}

		for _, ch := range workerJobs {
			close(ch)
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
		dur, err := b.repo.GetCPUUsage(ctx, j.Hostname, j.StartTime, j.EndTime)
		if err != nil {
			out <- result{failed: true}
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

	slices.Sort(durations)
	min := durations[0]
	max := durations[len(durations)-1]
	median := durations[len(durations)/2]
	if len(durations)%2 == 0 {
		median = (durations[(len(durations)/2)-1] + durations[len(durations)/2]) / 2
	}

	return Summary{
		MinDuration:    min,
		MaxDuration:    max,
		AvgDuration:    total / time.Duration(len(durations)),
		MedianDuration: median,
		TotalTime:      total,
		TotalCount:     len(durations) + failedCount,
		FailedCount:    failedCount,
	}
}

func getChannelIndex(hostname string, maxWorkers int) int {
	h := fnv.New32a()
	h.Write([]byte(hostname))
	s := int(h.Sum32())

	return s % maxWorkers
}
