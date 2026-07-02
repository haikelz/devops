package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Client struct {
	DB *sql.DB
}

func New(dsn string) (*Client, error) {
	db, err := sql.Open("pdx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Client{DB: db}, nil
}
