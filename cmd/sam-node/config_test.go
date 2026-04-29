package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalPolicy(t *testing.T) {
	// 1. Test valid YAML
	validYAML := `
version: "v1alpha1"
attenuation:
  policies:
    - 'deny if user("untrusted_sub_id");'
  checks:
    - 'check if time($time), $time < 2026-12-31T00:00:00Z;'
`
	dir := t.TempDir()
	validFile := filepath.Join(dir, "local_policy.yaml")
	if err := os.WriteFile(validFile, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadLocalPolicy(validFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(config.Policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(config.Policies))
	}
	if len(config.Checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(config.Checks))
	}

	// 2. Test missing file
	missingFile := filepath.Join(dir, "nonexistent.yaml")
	config, err = LoadLocalPolicy(missingFile)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	if len(config.Policies) != 0 {
		t.Errorf("expected empty policies, got %d", len(config.Policies))
	}
}
