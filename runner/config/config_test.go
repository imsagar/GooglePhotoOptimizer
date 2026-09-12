package config_test

import (
	"testing"

	"github.com/user/gpoptimizer/runner/config"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := &config.Config{
		ServerURL:   "https://gpoptimizer.example.com",
		Token:       "abc123",
		StoragePath: "~/GooglePhotosOptimized",
	}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *got != *want {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error loading config that was never saved")
	}
}
