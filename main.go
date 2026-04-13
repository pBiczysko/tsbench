package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pBiczysko/tsbench/bench"
	"github.com/pBiczysko/tsbench/csvstream"
)

type config struct {
	filePath string
	workers  int
}

const usage = `Usage: tsbench [flags]

Input modes:
  - Use --file <path> to read CSV from a file.
  - When --file is not specified, tsbench reads CSV from standard input

Examples:
  tsbench --file data.csv
  cat data.csv | tsbench

Flags:`

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("main: error: %v", err)
	}
}

func run(ctx context.Context) error {
	var cfg config

	flag.StringVar(&cfg.filePath, "file", "", "CSV file path with query parameters")
	flag.IntVar(&cfg.workers, "workers", 3, "number of workers to run the queries")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	var input io.Reader
	input = os.Stdin

	if cfg.filePath != "" {
		f, err := os.Open(cfg.filePath)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()

		input = f
	}

	jobs := make(chan bench.JobParams, 10)

	go func() {
		err := csvstream.ReadInto(ctx, input, jobs)
		if err != nil {
			// TODO: deal with panic
			panic(err)
		}
		close(jobs)
	}()
	m := mock{}
	b := bench.New(jobs, cfg.workers, m)

	summary := b.Process(ctx)
	fmt.Println(summary)

	return nil
}

type mock struct{}

func (m mock) GetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error) {
	return time.Duration(rand.Int63n(100) + 1), nil
}
