package migration

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

const (
	EnvMigrationsDir = "MIGRATIONS_DIR"
)

func ApplySQLMigrations(db *gorm.DB) error {
	if db == nil {
		return errors.New("nil mysql db")
	}

	migrationDir, err := resolveMigrationDir()
	if err != nil {
		return err
	}

	files, err := listUpMigrationFiles(migrationDir)
	if err != nil {
		return err
	}

	for _, filePath := range files {
		sqlContent, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return readErr
		}
		statements := splitSQLStatements(string(sqlContent))
		for _, stmt := range statements {
			if execErr := db.Exec(stmt).Error; execErr != nil {
				return execErr
			}
		}
	}

	return nil
}

func resolveMigrationDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(EnvMigrationsDir)); configured != "" {
		if stat, err := os.Stat(configured); err == nil && stat.IsDir() {
			return configured, nil
		}
	}

	candidates := []string{
		"server/migrations",
		"migrations",
		"../migrations",
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
	}

	return "", errors.New("sql migrations directory not found")
}

func listUpMigrationFiles(migrationDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			files = append(files, filepath.Join(migrationDir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func splitSQLStatements(content string) []string {
	segments := strings.Split(content, ";")
	statements := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		statements = append(statements, trimmed)
	}
	return statements
}
