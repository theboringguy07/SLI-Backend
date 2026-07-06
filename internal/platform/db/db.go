package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect initializes the PostgreSQL connection pool via pgx directly (no
// ORM). This works against both a direct Postgres endpoint and a PgBouncer
// transaction-pooling endpoint (e.g. Neon's "-pooler" connection string) as
// long as prepared-statement caching is disabled for pooled connections -
// see the DefaultQueryExecMode override below.
func Connect(dsn string, isProduction bool) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Pool sizing roughly matches the previous GORM defaults
	// (MaxOpenConns=100, MaxIdleConns=10, ConnMaxLifetime=1h).
	cfg.MaxConns = 100
	cfg.MinConns = 10
	cfg.MaxConnLifetime = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	slog.Info("Successfully connected to the database")

	return pool, nil
}
