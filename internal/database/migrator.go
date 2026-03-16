package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"database/sql"
)

// Migrate applies all SQL migration files from the migrations directory
func Migrate(db *sql.DB) error {
	migrationDir := "./migrations"
	err := filepath.WalkDir(migrationDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		_, err = db.Exec(string(sqlBytes))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", path, err)
		}

		log.Printf("Applied migration: %s", d.Name())
		return nil
	})

	return err
}
