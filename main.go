package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pBiczysko/tsbench/bench"
	"github.com/pBiczysko/tsbench/csvstream"
	"github.com/pBiczysko/tsbench/repo"
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

	pool, err := pgxpool.New(ctx, "postgres://postgres:password@127.0.0.1:5432/homework")
	if err != nil {
		return fmt.Errorf("creating db pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("running ping on db: %w", err)
	}
	repo := repo.New(pool, 0)

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

	jobs := make(chan bench.JobParams, 201)

	go func() {
		err := csvstream.ReadInto(ctx, input, jobs)
		if err != nil {
			// TODO: deal with panic
			panic(err)
		}
		close(jobs)
	}()

	b := bench.New(jobs, cfg.workers, repo)

	summary := b.Process(ctx)
	fmt.Println(summary)

	return nil
}
