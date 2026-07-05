package migrations

import "embed"

// FS holds the goose SQL migrations, embedded so cmd/migrate can apply them
// without shipping loose files.
//
//go:embed *.sql
var FS embed.FS
