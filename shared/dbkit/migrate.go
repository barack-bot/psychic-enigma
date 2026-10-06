package dbkit

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migration represents a single SQL migration file.
type Migration struct {
	Name string
	SQL  string
}

// LoadMigrations reads all .sql files from a directory, sorted by name.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{Name: entry.Name(), SQL: string(data)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Name < migrations[j].Name })
	return migrations, nil
}

// ApplyMigrations runs all migrations in order within a transaction.
func ApplyMigrations(ctx context.Context, db *sql.DB, migrations []Migration) error {
	for _, migration := range migrations {
		if _, err := db.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}
	}
	return nil
}