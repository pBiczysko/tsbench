// Package worker provides a way to run queries against DB using on job parameters provided via input channel.
// The result of job processing are sent on results channel.
package worker

import (
	"context"
	"time"
)

type JobFunc func(params JobParams) (time.Duration, error)

type JobParams struct {
	Hostname  string
	StartTime time.Time
	EndTime   time.Time
}

type Result struct {
	Duration time.Duration
	Failed   bool
}

type Worker struct {
	jobFn   JobFunc
	jobs    <-chan JobParams
	results chan<- Result
}

func New(jobs <-chan JobParams, results chan<- Result, jobFn JobFunc) Worker {
	return Worker{
		jobs:    jobs,
		jobFn:   jobFn,
		results: results,
	}
}

func (w Worker) Process(ctx context.Context) {
	for j := range w.jobs {
		d, err := w.jobFn(j)
		if err != nil {
			w.results <- Result{Failed: true}
			continue
		}

		w.results <- Result{Duration: d, Failed: false}
	}
}
