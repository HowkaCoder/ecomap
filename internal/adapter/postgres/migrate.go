package postgres

import (
	"context"
	"embed"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema/*.sql
var schemaFiles embed.FS

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.Glob(schemaFiles, "schema/*.sql")
	if err != nil { return err }
	sort.Strings(entries)
	for _, name := range entries {
		query, err := schemaFiles.ReadFile(name)
		if err != nil { return err }
		if _, err := pool.Exec(ctx, string(query)); err != nil { return err }
	}
	return nil
}
