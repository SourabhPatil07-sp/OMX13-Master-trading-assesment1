package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func ConnectDB() error {
	// The FASTAPI database URL typically looks like postgresql+asyncpg://user:pass@host/db
	// But pgxpool needs postgresql://user:pass@host/db
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres:postgres@localhost:5432/omnimarket?sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("unable to parse database url: %w", err)
	}

	// Optimize pool settings for high-frequency matching
	config.MaxConns = 50
	config.MinConns = 10

	Pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}

	err = Pool.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to the database")
	return nil
}

func CloseDB() {
	if Pool != nil {
		Pool.Close()
	}
}
