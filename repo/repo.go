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

// MeasureGetCPUUsage acquires a connection from the pool, marks the starting time and then executes the
// getCPUUsage query. On success it will return the query duration.
func (r *Repo) MeasureGetCPUUsage(ctx context.Context, hostname string, start, end time.Time) (time.Duration, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquiring db connection: %w", err)
	}
	defer conn.Release()

	// Start measuring time after connection was acquired.
	t := time.Now()
	if _, err := r.getCPUUsage(ctx, conn, hostname, start, end); err != nil {
		return 0, fmt.Errorf("executing getCPUUsage query: %w", err)
	}

	return time.Since(t), nil
}

type usageBucketCPU struct {
	bucket time.Time
	maxCPU float64
	minCPU float64
}

func (r *Repo) getCPUUsage(ctx context.Context, conn *pgxpool.Conn, hostname string, start, end time.Time) ([]usageBucketCPU, error) {
	const q = `
		SELECT
		    time_bucket ('1 minute', ts) AS bucket,
		    MAX(usage) AS max_cpu,
		    MIN(usage) AS min_cpu
		FROM
		    cpu_usage
		WHERE
		    host = $1
		    AND ts > $2::TIMESTAMPTZ
		    AND ts < $3::TIMESTAMPTZ
		GROUP BY
		    bucket
		ORDER BY
		    bucket;
		
		`

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := conn.Query(ctx, q, hostname, start, end)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	var out []usageBucketCPU
	for rows.Next() {
		var u usageBucketCPU
		if err := rows.Scan(&u.bucket, &u.maxCPU, &u.minCPU); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		out = append(out, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return out, nil
}
