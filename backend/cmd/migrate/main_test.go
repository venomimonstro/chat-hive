package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMigrationsOrdersByVersion(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "000010_ten.up.sql", "SELECT 10;")
	writeMigration(t, dir, "000002_two.up.sql", "SELECT 2;")
	writeMigration(t, dir, "000001_one.up.sql", "SELECT 1;")
	writeMigration(t, dir, "000001_one.down.sql", "SELECT -1;")

	items, err := loadMigrations(dir)
	if err != nil {
		t.Fatalf("loadMigrations returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 up migrations, got %d", len(items))
	}
	versions := []string{items[0].version, items[1].version, items[2].version}
	expected := []string{"000001", "000002", "000010"}
	for i := range expected {
		if versions[i] != expected[i] {
			t.Fatalf("unexpected order: got %v want %v", versions, expected)
		}
		if items[i].checksum == "" {
			t.Fatalf("migration %s has empty checksum", items[i].filename)
		}
	}
}

func TestLoadMigrationsRejectsDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "000001_first.up.sql", "SELECT 1;")
	writeMigration(t, dir, "000001_second.up.sql", "SELECT 2;")

	if _, err := loadMigrations(dir); err == nil {
		t.Fatal("expected duplicate version error")
	}
}

func TestLoadMigrationsRejectsInvalidFilename(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "bad_name.up.sql", "SELECT 1;")

	if _, err := loadMigrations(dir); err == nil {
		t.Fatal("expected invalid filename error")
	}
}

func writeMigration(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write migration %s: %v", name, err)
	}
}
