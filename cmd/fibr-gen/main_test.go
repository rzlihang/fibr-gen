package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	outputDir := filepath.Join(dir, "output")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	templatePath := filepath.Join(templateDir, "template.xlsx")
	f := excelize.NewFile()
	if err := f.SaveAs(templatePath); err != nil {
		t.Fatalf("save template: %v", err)
	}

	configPath := filepath.Join(dir, "config.yaml")
configContent := `workbook:
  id: "wb1"
  name: "Report"
  template: "template.xlsx"
  outputDir: "out"
  sheets:
    - name: "Sheet1"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dataSourcePath := filepath.Join(dir, "datasources.yaml")
	dataSourceContent := `dataSources:
  - name: "source1"
    driver: "mysql"
    dsn: "root:pass@tcp(localhost:3306)/db"
`
	if err := os.WriteFile(dataSourcePath, []byte(dataSourceContent), 0644); err != nil {
		t.Fatalf("write data sources: %v", err)
	}

	var logs bytes.Buffer
	if err := run(&logs, []string{
		"-config", configPath,
		"-datasources", dataSourcePath,
		"-templates", templateDir,
		"-output", outputDir,
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}

	outputPath := filepath.Join(outputDir, "out", "Report.xlsx")
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected output file, got error: %v", err)
	}
}

func TestValidate_CleanConfig(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}

	templatePath := filepath.Join(templateDir, "template.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "hello")
	if err := f.SaveAs(templatePath); err != nil {
		t.Fatalf("save template: %v", err)
	}
	f.Close()

	configPath := filepath.Join(dir, "config.yaml")
	configContent := `
workbook:
  id: "wb1"
  name: "Report"
  template: "template.xlsx"
  outputDir: "out"
  sheets:
    - name: "Sheet1"
      blocks:
        - name: "b1"
          type: "value"
          range:
            ref: "A1:B2"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	err := run(&logs, []string{"validate", "-c", configPath, "-t", templateDir})
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, logs.String())
	}
	output := logs.String()
	if !strings.Contains(output, "0 error") {
		t.Errorf("expected '0 error' in output, got:\n%s", output)
	}
}

func TestValidate_MissingSheet_ReportsError(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}

	templatePath := filepath.Join(templateDir, "template.xlsx")
	f := excelize.NewFile()
	if err := f.SaveAs(templatePath); err != nil {
		t.Fatalf("save template: %v", err)
	}
	f.Close()

	configPath := filepath.Join(dir, "config.yaml")
	configContent := `
workbook:
  id: "wb1"
  name: "Report"
  template: "template.xlsx"
  outputDir: "out"
  sheets:
    - name: "MissingSheet"
      blocks: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	err := run(&logs, []string{"validate", "-c", configPath, "-t", templateDir})
	if err == nil {
		t.Fatalf("expected error for missing sheet, got nil\noutput: %s", logs.String())
	}
	output := logs.String()
	if !strings.Contains(output, "ERROR") {
		t.Errorf("expected ERROR in output, got:\n%s", output)
	}
}
