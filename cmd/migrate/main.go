// Command migrate applies the embedded goose migrations to DATABASE_URL.
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down     # roll back the last migration
//	go run ./cmd/migrate status   # show migration status
//
// Point DATABASE_URL at the source or a branch as needed.
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"pg-branching-unikraft/migrations"
)

func main() {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}
	if err := goose.Run(command, sqlDB, "."); err != nil {
		log.Fatalf("goose %s: %v", command, err)
	}
}
