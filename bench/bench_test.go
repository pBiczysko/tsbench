package bench

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestProcess(t *testing.T) {
	type check func(t *testing.T, got Summary)

	hasSummary := func(want Summary) check {
		return func(t *testing.T, got Summary) {
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("summary mismatch (-got +want):\n%s", diff)
			}
		}
	}

	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name          string
		ctx           context.Context
		jobs          []JobParams
		repoResponses map[string]repoResponse
		checks        []check
	}{
		{
			name:          "no_jobs",
			jobs:          []JobParams{},
			repoResponses: map[string]repoResponse{},
			checks: []check{
				hasSummary(Summary{}),
			},
		},
		{
			name: "all_pass",
			ctx:  context.Background(),
			jobs: []JobParams{
				{Hostname: "a"},
				{Hostname: "b"},
				{Hostname: "c"},
			},
			repoResponses: map[string]repoResponse{
				"a": {duration: 50 * time.Millisecond},
				"b": {duration: 30 * time.Millisecond},
				"c": {duration: 10 * time.Millisecond},
			},
			checks: []check{
				hasSummary(Summary{
					MinDuration:    10 * time.Millisecond,
					MaxDuration:    50 * time.Millisecond,
					AvgDuration:    30 * time.Millisecond,
					MedianDuration: 30 * time.Millisecond,
					TotalTime:      90 * time.Millisecond,
					TotalCount:     3,
					FailedCount:    0,
				}),
			},
		},
		{
			name: "even_count_median",
			ctx:  context.Background(),
			jobs: []JobParams{
				{Hostname: "a"},
				{Hostname: "b"},
				{Hostname: "c"},
				{Hostname: "d"},
			},
			repoResponses: map[string]repoResponse{
				"a": {duration: 10 * time.Millisecond},
				"b": {duration: 20 * time.Millisecond},
				"c": {duration: 30 * time.Millisecond},
				"d": {duration: 40 * time.Millisecond},
			},
			checks: []check{
				hasSummary(Summary{
					MinDuration:    10 * time.Millisecond,
					MaxDuration:    40 * time.Millisecond,
					AvgDuration:    25 * time.Millisecond,
					MedianDuration: 25 * time.Millisecond, //(20 + 30)/2
					TotalTime:      100 * time.Millisecond,
					TotalCount:     4,
					FailedCount:    0,
				}),
			},
		},
		{
			name: "one_fails",
			ctx:  context.Background(),
			jobs: []JobParams{
				{Hostname: "a"},
				{Hostname: "b"},
				{Hostname: "c"},
			},
			repoResponses: map[string]repoResponse{
				"a": {duration: 50 * time.Millisecond},
				"b": {duration: 30 * time.Millisecond},
				"c": {err: errors.New("failed to execute query")},
			},
			checks: []check{
				hasSummary(Summary{
					MinDuration:    30 * time.Millisecond,
					MaxDuration:    50 * time.Millisecond,
					AvgDuration:    40 * time.Millisecond,
					MedianDuration: 40 * time.Millisecond,
					TotalTime:      80 * time.Millisecond,
					TotalCount:     3,
					FailedCount:    1,
				}),
			},
		},
		{
			name: "all_fail",
			ctx:  context.Background(),
			jobs: []JobParams{
				{Hostname: "a"},
				{Hostname: "b"},
				{Hostname: "c"},
			},
			repoResponses: map[string]repoResponse{
				"a": {err: errors.New("failed to execute query")},
				"b": {err: errors.New("failed to execute query")},
				"c": {err: errors.New("failed to execute query")},
			},
			checks: []check{
				hasSummary(Summary{
					TotalCount:  3,
					FailedCount: 3,
				}),
			},
		},
		{
			name: "context_cancelled",
			ctx:  cancelledContext,
			jobs: []JobParams{
				{Hostname: "a"},
				{Hostname: "b"},
				{Hostname: "c"},
			},
			repoResponses: map[string]repoResponse{
				"a": {duration: 10 * time.Millisecond},
				"b": {duration: 10 * time.Millisecond},
				"c": {duration: 10 * time.Millisecond},
			},
			checks: []check{
				hasSummary(Summary{
					TotalCount:  3,
					FailedCount: 3,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := make(chan JobParams, len(tt.jobs))
			for _, job := range tt.jobs {
				jobs <- job
			}
			close(jobs)
			repo := mockRepo{tt.repoResponses}
			log := slog.New(slog.NewTextHandler(os.Stderr, nil))
			bench := New(jobs, 2, repo, log)
			summary := bench.Process(tt.ctx)

			for _, ch := range tt.checks {
				ch(t, summary)
			}
		})
	}
}

type repoResponse struct {
	duration time.Duration
	err      error
}

type mockRepo struct {
	resp map[string]repoResponse
}

func (m mockRepo) MeasureGetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if resp, ok := m.resp[hostname]; ok {
		return resp.duration, resp.err
	}

	return 100 * time.Millisecond, nil
}
