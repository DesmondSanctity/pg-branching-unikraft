package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"pg-branching-unikraft/internal/api"
	"pg-branching-unikraft/internal/db"
)

func main() {
	ctx := context.Background()

	pool, err := db.Pool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := "0.0.0.0:" + port

	log.Printf("notes API listening on %s", addr)
	if err := http.ListenAndServe(addr, api.Router(&api.Handlers{Pool: pool})); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
