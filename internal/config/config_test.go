package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFrom_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := loadFrom(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults", cfg)
	}
}

func TestLoadFrom_PartialOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "refreshInterval: 5s\nmaxLogLines: 9000\nsplitRatio: 3\nallTailLines: 80\nmouseEnabled: false\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", cfg.RefreshInterval)
	}
	if cfg.MaxLogLines != 9000 {
		t.Errorf("MaxLogLines = %d, want 9000", cfg.MaxLogLines)
	}
	if cfg.SplitRatio != 3 {
		t.Errorf("SplitRatio = %d, want 3", cfg.SplitRatio)
	}
	if cfg.AllTailLines != 80 {
		t.Errorf("AllTailLines = %d, want 80", cfg.AllTailLines)
	}
	if cfg.MouseEnabled {
		t.Error("MouseEnabled = true, want explicit false honored")
	}
	// Untouched keys keep defaults.
	if cfg.DefaultTailLines != Default().DefaultTailLines {
		t.Errorf("DefaultTailLines = %d, want default", cfg.DefaultTailLines)
	}
	if cfg.LogBatchInterval != Default().LogBatchInterval {
		t.Errorf("LogBatchInterval = %v, want default", cfg.LogBatchInterval)
	}
}

func TestLoadFrom_SplitRatioClamped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("splitRatio: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SplitRatio != 5 {
		t.Errorf("SplitRatio = %d, want clamped to 5", cfg.SplitRatio)
	}

	if err := os.WriteFile(path, []byte("splitRatio: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = loadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SplitRatio != 1 {
		t.Errorf("SplitRatio = %d, want clamped to 1", cfg.SplitRatio)
	}
}

func TestLoadFrom_BadDurationErrorsWithDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("refreshInterval: banana\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFrom(path)
	if err == nil {
		t.Fatal("expected error for bad duration")
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults on error", cfg)
	}
}

func TestLoadFrom_BadYAMLErrorsWithDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(":\n\t- ["), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFrom(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if cfg != Default() {
		t.Errorf("cfg = %+v, want defaults on error", cfg)
	}
}
