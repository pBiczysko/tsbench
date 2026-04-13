package bench

import (
	"fmt"
	"time"
)

// JobParams contains the parameters required to run a job.
type JobParams struct {
	Hostname  string
	StartTime time.Time
	EndTime   time.Time
}

// Summary contains the statistics displayed to the caller.
type Summary struct {
	MinDuration    time.Duration
	MaxDuration    time.Duration
	AvgDuration    time.Duration
	MedianDuration time.Duration
	TotalTime      time.Duration
	TotalCount     int
	FailedCount    int
}

// String returns a human-readable representation of Summary.
func (s Summary) String() string {
	format := `
Benchmark summary
    Total count:      %d
    Total time:       %v
    Failed count:     %d
    Min duration:     %v
    Max duration:     %v
    Avg duration:     %v
    Median duration:  %v
`
	return fmt.Sprintf(
		format,
		s.TotalCount,
		s.TotalTime,
		s.FailedCount,
		s.MinDuration,
		s.MaxDuration,
		s.AvgDuration,
		s.MedianDuration,
	)
}

type result struct {
	failed   bool
	duration time.Duration
}
