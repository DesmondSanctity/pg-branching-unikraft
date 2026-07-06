// Command seed populates the database with realistic, uneven multi-tenant data:
// at least 20 tenants, each with a randomized 10–200 notes. Run it against the
// deployed instance (set DATABASE_URL) — the whole point is branching from a
// realistic data shape, not a uniform toy table.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"pg-branching-unikraft/internal/db"
)

func main() {
	ctx := context.Background()

	pool, err := db.Pool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	const tenantCount = 25
	totalNotes := 0

	for i := 0; i < tenantCount; i++ {
		name := fmt.Sprintf("Tenant %02d — %s", i+1, randCompany())
		var tenantID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO tenants (name) VALUES ($1) RETURNING id::text`, name,
		).Scan(&tenantID); err != nil {
			log.Fatalf("insert tenant: %v", err)
		}

		n := noteCount()
		batch := make([][]any, 0, n)
		for j := 0; j < n; j++ {
			batch = append(batch, []any{
				tenantID,
				fmt.Sprintf("Note %d for tenant %d", j+1, i+1),
				randBody(),
			})
		}
		for _, row := range batch {
			if _, err := pool.Exec(ctx,
				`INSERT INTO notes (tenant_id, title, body) VALUES ($1, $2, $3)`,
				row...,
			); err != nil {
				log.Fatalf("insert note: %v", err)
			}
		}
		totalNotes += n
		log.Printf("tenant %d/%d: %d notes", i+1, tenantCount, n)
	}

	log.Printf("seed complete: %d tenants, %d notes total", tenantCount, totalNotes)
}

// noteCount returns a randomized per-tenant note count in the inclusive range
// 10..200, so the seeded data is deliberately uneven.
func noteCount() int {
	return 10 + rand.Intn(191)
}

func randCompany() string {
	words := []string{"Acme", "Globex", "Initech", "Umbrella", "Hooli", "Stark", "Wayne", "Wonka", "Cyberdyne", "Soylent"}
	return words[rand.Intn(len(words))]
}

func randBody() string {
	lines := []string{
		"Follow up with the customer about the open ticket.",
		"Draft the quarterly report and circulate for review.",
		"Investigate the latency spike from last night.",
		"Schedule the migration window with the on-call team.",
		"Update the runbook with the new failover steps.",
	}
	return lines[rand.Intn(len(lines))]
}
