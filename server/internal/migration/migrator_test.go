package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMigrationDirPrefersEnvWhenValid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(EnvMigrationsDir, tmpDir)

	dir, err := resolveMigrationDir()
	if err != nil {
		t.Fatalf("resolve migration dir with env failed: %v", err)
	}
	if dir != tmpDir {
		t.Fatalf("resolve migration dir = %q, want %q", dir, tmpDir)
	}
}

func TestResolveMigrationDirReturnsErrorWhenMissing(t *testing.T) {
	t.Setenv(EnvMigrationsDir, filepath.Join(t.TempDir(), "missing"))

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	isolated := t.TempDir()
	if chdirErr := os.Chdir(isolated); chdirErr != nil {
		t.Fatalf("chdir failed: %v", chdirErr)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	_, err = resolveMigrationDir()
	if err == nil {
		t.Fatalf("expected error when migration dir missing")
	}
}

func TestListUpMigrationFilesSorted(t *testing.T) {
	dir := t.TempDir()
	files := []string{"0002_add_idx.up.sql", "0001_init.up.sql", "notes.txt", "0001_init.down.sql"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatalf("write file %s failed: %v", name, err)
		}
	}

	given, err := listUpMigrationFiles(dir)
	if err != nil {
		t.Fatalf("list up migration files failed: %v", err)
	}

	if len(given) != 2 {
		t.Fatalf("up migration file count = %d, want 2", len(given))
	}
	if filepath.Base(given[0]) != "0001_init.up.sql" {
		t.Fatalf("first up migration = %q, want 0001_init.up.sql", filepath.Base(given[0]))
	}
	if filepath.Base(given[1]) != "0002_add_idx.up.sql" {
		t.Fatalf("second up migration = %q, want 0002_add_idx.up.sql", filepath.Base(given[1]))
	}
}
