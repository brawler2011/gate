package pkg

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	maxConns          = 60
	minConns          = 10
	maxConnLifetime   = 120 * time.Second
	maxConnIdleTime   = 20 * time.Second
	healthCheckPeriod = 30 * time.Second
)

func NewPostgresDB(dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	config.MaxConns = maxConns
	config.MinConns = minConns
	config.MaxConnLifetime = maxConnLifetime
	config.MaxConnIdleTime = maxConnIdleTime
	config.HealthCheckPeriod = healthCheckPeriod

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// NewPostgresDBForMigrations returns a sql.DB for use with goose migrations
func NewPostgresDBForMigrations(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
