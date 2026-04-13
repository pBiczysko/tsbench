package csvstream_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pBiczysko/tsbench/csvstream"
)

func TestReadInto(t *testing.T) {
	type check func(t *testing.T, params []csvstream.JobParams, err error)

	hasNoError := func() check {
		return func(t *testing.T, _ []csvstream.JobParams, err error) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}

	hasError := func(expError error) check {
		return func(t *testing.T, _ []csvstream.JobParams, err error) {
			if !errors.Is(err, expError) {
				t.Fatalf("expected error to be % v got: %v", expError, err)
			}
		}
	}

	hasJobParams := func(expParams []csvstream.JobParams) check {
		return func(t *testing.T, params []csvstream.JobParams, _ error) {
			if diff := cmp.Diff(params, expParams); diff != "" {
				t.Errorf("result mismatch (-got +want):\n%s", diff)
			}
		}
	}

	tests := []struct {
		name   string
		input  io.Reader
		checks []check
	}{
		{
			name:  "happy_path",
			input: linesReader("hostname,start_time,end_time", "host_000008,2017-01-01 08:59:22,2017-01-01 09:59:22"),
			checks: []check{
				hasNoError(),
				hasJobParams([]csvstream.JobParams{
					{
						Hostname:  "host_000008",
						StartTime: time.Date(2017, 1, 1, 8, 59, 22, 0, time.UTC),
						EndTime:   time.Date(2017, 1, 1, 9, 59, 22, 0, time.UTC),
					},
				}),
			},
		},
		{
			name:  "invalid_header_hostname",
			input: linesReader("invalid_host,start_time,end_time"),
			checks: []check{
				hasError(csvstream.ErrInvalidHeader),
			},
		},
		{
			name:  "invalid_header_start_time",
			input: linesReader("hostname,invalid_start_time,end_time"),
			checks: []check{
				hasError(csvstream.ErrInvalidHeader),
			},
		},
		{
			name:  "invalid_header_end_time",
			input: linesReader("hostname,start_time,invalid_end_time"),
			checks: []check{
				hasError(csvstream.ErrInvalidHeader),
			},
		},
		{
			name:  "invalid_row_hostname_missing",
			input: linesReader("hostname,start_time,end_time", ",2017-01-01 08:59:22,2017-01-01 09:59:22"),
			checks: []check{
				hasError(csvstream.ErrInvalidRow),
			},
		},
		{
			name:  "invalid_row_invalid_start_time_format",
			input: linesReader("hostname,start_time,end_time", "host_000008,2017-01-01,2017-01-01 09:59:22"),
			checks: []check{
				hasError(csvstream.ErrInvalidRow),
			},
		},
		{
			name:  "invalid_row_invalid_end_time_format",
			input: linesReader("hostname,start_time,end_time", "host_000008,2017-01-01 08:59:22,2017-01-01"),
			checks: []check{
				hasError(csvstream.ErrInvalidRow),
			},
		},
		{
			name:  "invalid_header_one_field_missing",
			input: linesReader("hostname,start_time"),
			checks: []check{
				hasError(csvstream.ErrReadingCSV),
			},
		},
		{
			name:  "invalid_csv_row_one_field_missing",
			input: linesReader("hostname,start_time,end_time", "host_000008,2017-01-01"),
			checks: []check{
				hasError(csvstream.ErrReadingCSV),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := make(chan csvstream.JobParams, 100)
			err := csvstream.ReadInto(context.Background(), tt.input, results)
			close(results)
			res := drainResults(results)

			for _, ch := range tt.checks {
				ch(t, res, err)
			}
		})
	}
}

func linesReader(lines ...string) *strings.Reader {
	return strings.NewReader(strings.Join(lines, "\n"))
}

func drainResults(in <-chan csvstream.JobParams) []csvstream.JobParams {
	out := make([]csvstream.JobParams, 0)
	for r := range in {
		out = append(out, r)
	}

	return out
}
