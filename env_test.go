package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# comment\nTEST_DOTENV_VALUE='hello world'\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("TEST_DOTENV_VALUE", "")
	if err := os.Unsetenv("TEST_DOTENV_VALUE"); err != nil {
		t.Fatalf("unset env: %v", err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("TEST_DOTENV_VALUE"); got != "hello world" {
		t.Fatalf("TEST_DOTENV_VALUE = %q", got)
	}
}

func TestLoadDotEnvDoesNotOverwriteEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TEST_DOTENV_PRIORITY=from-file\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("TEST_DOTENV_PRIORITY", "from-shell")
	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("TEST_DOTENV_PRIORITY"); got != "from-shell" {
		t.Fatalf("TEST_DOTENV_PRIORITY = %q", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
}
