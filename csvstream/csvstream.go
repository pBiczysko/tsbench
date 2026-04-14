// Package csvstream provides a mechanism to stream csv data from reader interface into channel.
package csvstream

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/pBiczysko/tsbench/bench"
)

var (
	ErrInvalidHeader = errors.New("invalid header")
	ErrInvalidRow    = errors.New("invalid row")
	ErrReadingCSV    = errors.New("error while reading csv")
)

// ReadInto takes a reader and a channel. It reads and validates the csv header and then goes row by row
// parses data in bench.JobParams and sends it onto out channel.
func ReadInto(ctx context.Context, in io.Reader, out chan<- bench.JobParams) error {
	r := csv.NewReader(in)
	r.ReuseRecord = true
	r.FieldsPerRecord = 3

	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("reading header: %w: %w", ErrReadingCSV, err)
	}

	if err := validateHeader(header); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidHeader, err)
	}

	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("reading row: %w: %w", ErrReadingCSV, err)
		}

		jp, err := toJobParams(record)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidRow, err)
		}

		select {
		case out <- jp:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func validateHeader(header []string) error {
	validateField := func(idx int, exp string) error {
		if header[idx] != exp {
			return fmt.Errorf("expected header field at index %d to be %q, got: %q", idx, exp, header[idx])
		}

		return nil
	}

	if err := validateField(0, "hostname"); err != nil {
		return err
	}

	if err := validateField(1, "start_time"); err != nil {
		return err
	}

	if err := validateField(2, "end_time"); err != nil {
		return err
	}

	return nil
}

func toJobParams(in []string) (bench.JobParams, error) {
	if in[0] == "" {
		return bench.JobParams{}, errors.New("hostname needs to be provided")
	}

	st, err := time.Parse(time.DateTime, in[1])
	if err != nil {
		return bench.JobParams{}, fmt.Errorf("parsing start_time %s: %w", in[1], err)
	}

	et, err := time.Parse(time.DateTime, in[2])
	if err != nil {
		return bench.JobParams{}, fmt.Errorf("parsing end_time %s: %w", in[2], err)
	}

	return bench.JobParams{
		Hostname:  in[0],
		StartTime: st,
		EndTime:   et,
	}, nil
}
