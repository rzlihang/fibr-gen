package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigBundle_Validation(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "fibr-gen-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Case 1: Valid Config
	validConfig := `
workbook:
  id: wb1
  name: Report
  template: tpl.xlsx
  outputDir: out
  sheets:
    - name: Sheet1
      blocks:
        - name: Block1
          type: value
          range: {ref: "A1"}
          dataView: view1

dataViews:
  - name: view1
    dataSource: ds1
    labels:
      - name: l1
        column: c1

dataSources:
  - name: ds1
    driver: mysql
    dsn: user:pass@tcp(localhost:3306)/db
`
	validPath := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write valid config: %v", err)
	}

	_, _, _, err = LoadConfigBundle(validPath)
	if err != nil {
		t.Errorf("LoadConfigBundle() valid config error = %v", err)
	}

	// Case 2: Invalid Config (Missing DataSource)
	invalidConfig := `
workbook:
  id: wb1
  name: Report
  template: tpl.xlsx
  outputDir: out
  sheets:
    - name: Sheet1
      blocks:
        - name: Block1
          type: value
          range: {ref: "A1"}
          dataView: view1

dataViews:
  - name: view1
    dataSource: unknown_ds # References unknown data source
    labels:
      - name: l1
        column: c1

dataSources:
  - name: ds1
    driver: mysql
    dsn: ...
`
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	_, _, _, err = LoadConfigBundle(invalidPath)
	if err == nil {
		t.Errorf("LoadConfigBundle() expected error for invalid config, got nil")
	} else if !strings.Contains(err.Error(), "unknown DataSource") {
		t.Errorf("LoadConfigBundle() error = %v, want error containing 'unknown DataSource'", err)
	}
}

func TestLoadConfigBundleRaw_SkipsValidation(t *testing.T) {
	dir := t.TempDir()
	// Config with intentionally empty DSN — strict loader rejects this
	content := `
workbook:
  id: "wb1"
  name: "Report"
  template: "tpl.xlsx"
  outputDir: "out"
  sheets:
    - name: "Sheet1"
dataViews:
  - name: "view1"
    dataSource: "src1"
    labels:
      - name: "emp_name"
        column: "name"
dataSources:
  - name: "src1"
    driver: "mysql"
    dsn: ""
`
	path := filepath.Join(dir, "bundle.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	wb, views, sources, err := LoadConfigBundleRaw(path)
	if err != nil {
		t.Fatalf("LoadConfigBundleRaw() unexpected error: %v", err)
	}
	if wb.Name != "Report" {
		t.Errorf("wb.Name = %q, want %q", wb.Name, "Report")
	}
	if _, ok := views["view1"]; !ok {
		t.Error("expected view1 in views")
	}
	if _, ok := sources["src1"]; !ok {
		t.Error("expected src1 in sources")
	}
}
