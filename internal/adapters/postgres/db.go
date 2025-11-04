package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// DB holds the connection pool.
type DB struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

// NewDB creates and tests a new database connection.
func NewDB(ctx context.Context, connString string, baseLogger *zerolog.Logger) (*DB, error) {
	log := baseLogger.With().Str("component", "postgres").Logger()

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse DB connection string")
		return nil, err
	}

	// You can configure pool size here
	// poolConfig.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create connection pool")
		return nil, err
	}

	// Ping the database to ensure a valid connection
	if err := pool.Ping(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to ping database")
		pool.Close() // Clean up
		return nil, err
	}

	log.Info().Msg("Database connection pool established")
	return &DB{pool: pool, log: log}, nil
}

// Close gracefully closes the connection pool.
func (db *DB) Close() {
	db.log.Info().Msg("Closing database connection pool")
	db.pool.Close()
}
