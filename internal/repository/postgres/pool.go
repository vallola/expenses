package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func NewPool(databaseURL string, log *zap.Logger) (*pgxpool.Pool, error) {
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Error("Failed to parse database connection config", zap.Error(err))
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Error("Failed to create pgxpool", zap.Error(err))
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		log.Error("Failed to ping the database pool", zap.Error(err))
		return nil, err
	}

	return pool, nil
}
