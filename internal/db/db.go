package db

import (
	"context"
	"errors"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoDSN is returned when DATABASE_URL is not set.
var ErrNoDSN = errors.New("DATABASE_URL is not set")

// Pool creates a pgx connection pool from the DATABASE_URL environment variable.
// The DSN must include TLS settings appropriate for the Unikraft Cloud Postgres
// FQDN (sslmode=require), e.g.:
//
//	postgres://postgres:<pw>@<fqdn>:5432/postgres?sslmode=require
func Pool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, ErrNoDSN
	}
	return pgxpool.New(ctx, dsn)
}
