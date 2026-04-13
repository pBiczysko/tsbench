package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pBiczysko/tsbench/csvstream"
)

type config struct {
	filePath string
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

	jobs := make(chan csvstream.JobParams, 10)

	go func() {
		err := csvstream.ReadInto(ctx, input, jobs)
		if err != nil {
			panic(err)
		}
		close(jobs)
	}()

	for r := range jobs {
		fmt.Println(r)
	}

	return nil
}
