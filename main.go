package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pBiczysko/tsbench/bench"
	"github.com/pBiczysko/tsbench/csvstream"
	"github.com/pBiczysko/tsbench/repo"
)

type config struct {
	filePath     string
	workers      int
	queryTimeout time.Duration
	database     string
}

const usage = `Usage: tsbench [flags]

Input modes:
  - Use --file <path> to read CSV from a file.
  - When --file is not specified, tsbench reads CSV piped through standard input

Examples:
  tsbench --file data.csv
  cat data.csv | tsbench

Flags:`

func main() {
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(ctx, log); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	var cfg config

	flag.StringVar(&cfg.filePath, "file", "", "CSV file path with query parameters")
	flag.IntVar(&cfg.workers, "workers", 3, "number of workers to run the queries")
	flag.DurationVar(&cfg.queryTimeout, "query-timeout", 100*time.Millisecond, "query execution timeout")
	flag.StringVar(&cfg.database, "database", "", "database connection string")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	input, err := getCSVinput(cfg.filePath)
	if err != nil {
		return fmt.Errorf("getting csv input: %w", err)
	}
	defer input.Close()

	dbPool, err := initDBConn(ctx, cfg.database, cfg.workers)
	if err != nil {
		return fmt.Errorf("initializing repository: %w", err)
	}
	defer dbPool.Close()

	repo := repo.New(dbPool, cfg.queryTimeout)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Jobs is a buffered channel with capacity equal to the number of workers.
	jobs := make(chan bench.JobParams, cfg.workers)
	b := bench.New(jobs, cfg.workers, repo, log)
	summary := make(chan bench.Summary, 1)
	go func() {
		summary <- b.Process(ctx)
	}()

	if err := csvstream.ReadInto(ctx, input, jobs); err != nil {
		close(jobs)
		// Wait for bench to finish.
		<-summary
		return fmt.Errorf("reading csv input: %w", err)
	}
	close(jobs)

	out := <-summary
	fmt.Fprint(os.Stdout, out)

	return nil
}

func validateConfig(cfg config) error {
	if cfg.database == "" {
		return errors.New("database connection string is required")
	}
	if cfg.workers <= 0 {
		return errors.New("number of workers need to be positive")
	}

	return nil
}

func getCSVinput(filePath string) (io.ReadCloser, error) {
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("opening file: %w", err)
		}

		return f, nil
	}
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("getting stdin stat: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("either --file flag needs to be set or data piped into stdin")
	}

	return io.NopCloser(bufio.NewReader(os.Stdin)), nil
}

func initDBConn(ctx context.Context, database string, workers int) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(database)
	if err != nil {
		return nil, fmt.Errorf("parsing db pool config: %w", err)
	}

	// Use number of workers as number of connections but limit it not to pass the max_connections.
	poolCfg.MaxConns = int32(min(workers, 20))
	poolCfg.MinConns = int32(min(workers, 20))
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running ping on db: %w", err)
	}

	return pool, nil
}
