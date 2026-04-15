# tsbench – TimescaleDB Query Benchmark CLI

`tsbench` is a Go cli tool that benchmarks the execution time of parameterised `SELECT` queries against a TimescaleDB instance. It reads query parameters from a CSV file (or standard input), dispatches jobs to configurable number of concurrent workers and emits a statistical summary upon completion. 

## Running 

### Prerequisites 
- Docker and Docker Compose 
- Go 1.26+ (for local builds)

### Configuration

`tsbench` is configured via CLI flags when running directly, or via environment variables when using Docker Compose. The environment variables map directly to the corresponding flags:

| Env Variable          | CLI Flag        | Description                             | Default                                              |
| --------------------- | --------------- | --------------------------------------- | ---------------------------------------------------- |
| `TSBENCH_DATABASE`      | `--database`      | TimescaleDB connection string           | `postgres://postgres:password@timescaledb:5432/homework` |
| `TSBENCH_FILE`          | `--file`          | Path to input CSV  | `./input/query_params.csv`                             |
| `TSBENCH_WORKERS`       | `--workers`       | Number of workers      | `3`                                                    |
| `TSBENCH_QUERY_TIMEOUT` | `--query-timeout` | Timeout per query      | `100ms`                                                |


> **Note:** The database connection string is passed as a plain CLI flag to keep the project simple and focused on benchmarking logic.



### Running with Docker Compose

To get up and running quickly:

```bash
docker compose up
```

This command: 
1. Starts a TimescaleDB instance.
2. Seeds it with the schema and sample data from the `seed` directory. 
3. Waits until the database is ready, then runs `tsbench` against the default input file (`input/query_params.csv`).
4. Prints the benchmark results and exits.

To override any of the defaults:

```bash
TSBENCH_WORKERS=8 TSBENCH_QUERY_TIMEOUT=200ms docker compose up
```

To run with stdin instead of a file:

```bash
cat input/query_params.csv | docker compose run -T tsbench --database "postgres://postgres:password@timescaledb:5432/homework"
```

> **note**: this uses `docker compose run` so the `--database` flag is passed directly 

### Running locally 

To run `tsbench` locally, you need to have a TimescaleDB instance running, either from an existing setup or by spinning one up using Docker. Ensure that the database schema matches the one in `/seed/cpu_usage.sql`.

Start TimescaleDB with Docker:

```bash
docker run -d \
  --name timescaledb \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  -v ./seed/cpu_usage.sql:/docker-entrypoint-initdb.d/cpu_usage.sql \
  -v ./seed/cpu_usage.csv:/seed/cpu_usage.csv \
  timescale/timescaledb:latest-pg18
```

Build the app:
```bash
go build -o tsbench .
```

Run against a file:
```bash
./tsbench --database "postgres://postgres:password@localhost:5432/homework" --file input/query_params.csv
```

or with stdin:

```bash 
cat input/query_params.csv | ./tsbench --database "postgres://postgres:password@localhost:5432/homework"
```

### Testing

Packages are covered with unit test, to run them:

```bash
go test ./... -race
```

### Output example 

After a successful run, `tsbench` prints a statistical summary: 
```text
Benchmark summary
    Total count:      200
    Total time:       209.882794ms
    Failed count:     0
    Min duration:     614.542µs
    Max duration:     6.874792ms
    Avg duration:     1.049413ms
    Median duration:  786.875µs
```

## Design document 
See [DESIGN.md](./docs/DESIGN.md) for:
- High‑level overview of the architecture.  
- Description of key design decisions.  
- List of assumptions.