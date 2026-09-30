package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mardwerk/td-profile/internal/atlasvalidate"
)

func TestCLI(t *testing.T) {
	data := t.TempDir()
	if err := os.Mkdir(filepath.Join(data, "Maps"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "Maps", "test.json"), []byte(`{"kind":"map","id":"test","difficulty":"easy"}`), 0644); err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	source := filepath.Join("..", "..", "profile")
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destination := filepath.Join(profile, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}

	// This CLI fixture checks only collection integrity, without a Tower corpus.
	if err := os.WriteFile(filepath.Join(profile, "classifications.json"), []byte(`{"groups":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	rawRules, err := os.ReadFile(filepath.Join(profile, "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rules map[string]any
	if err := json.Unmarshal(rawRules, &rules); err != nil {
		t.Fatal(err)
	}
	rules["rules"] = []any{}
	rawRules, err = json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "rules.json"), rawRules, 0644); err != nil {
		t.Fatal(err)
	}
	base := []string{"--data", data, "--profile", profile}
	var stdout, stderr bytes.Buffer
	if status := run(base, &stdout, &stderr); status != 0 {
		t.Fatalf("CLI exit=%d, output=%s, stderr=%s", status, stdout.String(), stderr.String())
	}
	var report atlasvalidate.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || !report.Valid || report.Profile == nil || report.Checker.Version == "" || report.Checker.Name != "validator" {
		t.Fatalf("invalid report: %s", stdout.String())
	}
	stdout.Reset()
	if status := run(append(base, "--format", "text"), &stdout, &stderr); status != 0 || !strings.Contains(stdout.String(), "valid=true; integrity=true; rules=true") {
		t.Fatalf("invalid text output: %s", stdout.String())
	}
	stdout.Reset()
	if status := run(append(base, "--relations"), &stdout, &stderr); status != 0 || json.Unmarshal(stdout.Bytes(), &report) != nil {
		t.Fatalf("relations output failed: %s", stdout.String())
	}
	if err := os.WriteFile(filepath.Join(data, "Maps", "test.json"), []byte(`{"kind":"map","id":"test","difficulty":false}`), 0644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if status := run(base, &stdout, &stderr); status != 1 || json.Unmarshal(stdout.Bytes(), &report) != nil || report.Valid {
		t.Fatalf("invalid data did not exit 1: %s", stdout.String())
	}
}

func TestCLIArguments(t *testing.T) {
	cases := [][]string{
		nil,
		{"--data", "data"},
		{"--data", "data", "--profile", "profile", "--format", "xml"},
		{"--data", "data", "--profile", "profile", "--format", "text", "--relations"},
		{"--data", "data", "--profile", "profile", "extra"},
		{"--unknown"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if status := run(args, &stdout, &stderr); status != 2 {
			t.Errorf("args=%v exit=%d", args, status)
		}
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, &stdout, &stderr); status != 0 {
		t.Errorf("help exit=%d", status)
	}
}
