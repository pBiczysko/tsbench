// Package repo provides a way to execute queries against TimescaleDB
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func New(pool *pgxpool.Pool, timeout time.Duration) *Repo {
	return &Repo{
		pool:         pool,
		queryTimeout: timeout,
	}
}

func (r *Repo) GetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error) {
	const q = `
		SELECT
		    time_bucket ('1 minute', ts) AS bucket_minute,
		    MAX(cpu_usage) AS max_cpu,
		    MIN(cpu_usage) AS min_cpu
		FROM
		    cpu_usage
		WHERE
		    host= $1
		    AND ts > $2::TIMESTAMPTZ
		    AND ts < $3::TIMESTAMPTZ
		GROUP BY
		    bucket_minute
		ORDER BY
		    bucket_minute;
		`
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquiring db connection: %v", err)
	}
	defer conn.Release()

	t := time.Now()
	rows, err := conn.Query(ctx, q, hostname, start, end)
	if err != nil {
		fmt.Println(err)
		return 0, fmt.Errorf("executing query: %w", err)
	}

	// TODO: collect rows
	defer rows.Close()

	return time.Since(t), nil
}
