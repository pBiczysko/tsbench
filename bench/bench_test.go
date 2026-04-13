package bench

import (
	"context"
	"errors"
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

	tests := []struct {
		name          string
		jobs          []JobParams
		repoResponses map[string]repoResponse
		checks        []check
	}{
		{
			name: "all_pass",
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
			name: "one_fails",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := make(chan JobParams, len(tt.jobs))
			for _, job := range tt.jobs {
				jobs <- job
			}
			close(jobs)
			repo := mockRepo{tt.repoResponses}
			bench := New(jobs, 2, repo)
			summary := bench.Process(context.Background())

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

func (m mockRepo) GetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error) {
	if resp, ok := m.resp[hostname]; ok {
		return resp.duration, resp.err
	}

	return 100 * time.Millisecond, nil
}
