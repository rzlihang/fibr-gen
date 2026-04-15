# CLI Validate Subcommand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `fibr-gen validate` subcommand that cross-checks a YAML config against an Excel template and reports all issues without generating a report.

**Architecture:** Refactor the CLI entry point to dispatch subcommands, add a `LoadConfigBundleRaw` loader that parses without strict validation, and introduce a `TemplateValidator` in `core/` that opens the Excel template with excelize and checks sheet existence, range syntax, and placeholder coverage.

**Tech Stack:** Go stdlib, `github.com/xuri/excelize/v2` (already a dependency), `regexp` for placeholder scanning.

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/fibr-gen/main.go` | Modify | Dispatch: `validate` → `runValidate`, else → `runGenerate` |
| `cmd/fibr-gen/cmd_generate.go` | Create | `runGenerate()` — current `run()` body moved here |
| `cmd/fibr-gen/cmd_validate.go` | Create | `runValidate()` — loads config raw, runs TemplateValidator, prints issues |
| `config/loader.go` | Modify | Add `LoadConfigBundleRaw()` — parse-only, no validation |
| `config/loader_test.go` | Modify | Add test for `LoadConfigBundleRaw` |
| `core/validate.go` | Create | `TemplateValidator`, `ValidationIssue`, helpers |
| `core/validate_test.go` | Create | Unit tests for TemplateValidator |
| `cmd/fibr-gen/main_test.go` | Modify | Add `TestValidate_*` integration tests |

---

## Task 1: Subcommand Dispatch

Extract the current `run()` body into `cmd_generate.go` as `runGenerate()`, then make `run()` in `main.go` a thin dispatcher.

**Files:**
- Create: `cmd/fibr-gen/cmd_generate.go`
- Modify: `cmd/fibr-gen/main.go`

- [ ] **Step 1: Create `cmd_generate.go` with the extracted generate logic**

Create `cmd/fibr-gen/cmd_generate.go` with this exact content (the body is the current `run()` in `main.go`, renamed):

```go
package main

import (
	"context"
	"database/sql"
	"fibr-gen/config"
	"fibr-gen/core"
	"flag"
	"fmt"
	"io"
	"log/slog"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func runGenerate(output io.Writer, args []string) error {
	flags := flag.NewFlagSet("fibr-gen generate", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.Usage = func() {
		fmt.Fprintf(output, "Usage: fibr-gen generate [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}

	var (
		configFile     string
		dataSourceFile string
		templateDir    string
		outputDir      string
		fetcherType    string
		dbDSN          string
		csvDir         string
		s3Bucket       string
		s3Prefix       string
	)

	flags.StringVar(&configFile, "config", "./test/config.yaml", "Path to configuration bundle")
	flags.StringVar(&configFile, "c", "./test/config.yaml", "Path to configuration bundle (short)")
	flags.StringVar(&dataSourceFile, "datasources", "", "Path to data source bundle (optional)")
	flags.StringVar(&templateDir, "templates", "./test/templates", "Template group directory")
	flags.StringVar(&templateDir, "t", "./test/templates", "Template group directory (short)")
	flags.StringVar(&outputDir, "output", "./test/output", "Directory for output files")
	flags.StringVar(&outputDir, "o", "./test/output", "Directory for output files (short)")
	flags.StringVar(&fetcherType, "fetcher", "csv", "Data fetcher type: csv, dynamodb, mysql, postgres")
	flags.StringVar(&fetcherType, "f", "csv", "Data fetcher type (short)")
	flags.StringVar(&dbDSN, "db-dsn", "", "Database connection string (DSN) for mysql/postgres")
	flags.StringVar(&csvDir, "csv-dir", "./test/data_csv", "Directory containing CSV files for csv fetcher")
	flags.StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket name for uploading output")
	flags.StringVar(&s3Prefix, "s3-prefix", "fibr-gen-output", "S3 prefix (folder) for uploaded files")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	slog.Info("Loading configuration bundle", "file", configFile)
	wbConf, views, dataSources, err := config.LoadConfigBundle(configFile)
	if err != nil {
		return err
	}
	if dataSourceFile != "" {
		slog.Info("Loading data source bundle", "file", dataSourceFile)
		dataSources, err = config.LoadDataSourcesBundle(dataSourceFile)
		if err != nil {
			return err
		}
	}
	if len(dataSources) > 0 {
		slog.Info("Loaded data sources", "count", len(dataSources))
	}

	var fetcher core.DataFetcher

	switch fetcherType {
	case "dynamodb":
		slog.Info("Initializing DynamoDB Data Fetcher")
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)
		}
		fetcher = core.NewDynamoDBDataFetcher(cfg)
	case "mysql", "postgres":
		if dbDSN == "" {
			return fmt.Errorf("db-dsn is required for %s fetcher", fetcherType)
		}
		slog.Info("Initializing SQL Data Fetcher", "type", fetcherType)
		db, err := sql.Open(fetcherType, dbDSN)
		if err != nil {
			return fmt.Errorf("failed to open db connection: %w", err)
		}
		if err := db.Ping(); err != nil {
			return fmt.Errorf("failed to ping db: %w", err)
		}
		fetcher = core.NewSQLDataFetcher(db, fetcherType)
	default:
		slog.Info("Initializing CSV Data Fetcher", "dir", csvDir)
		fetcher = core.NewCsvDataFetcher(csvDir)
	}

	slog.Info("Processing Workbook", "name", wbConf.Name, "id", wbConf.Id)
	configRegistry := config.NewMemoryConfigRegistry(views, dataSources)
	ctx := core.NewGenerationContext(wbConf, configRegistry, fetcher, map[string]string{"env": "dev"})
	generator := core.NewGenerator(ctx)
	if err := generator.Generate(templateDir, outputDir); err != nil {
		return fmt.Errorf("generate workbook %s: %w", wbConf.Name, err)
	}
	slog.Info("Successfully generated", "name", wbConf.Name)

	if s3Bucket != "" {
		slog.Info("Starting S3 upload", "bucket", s3Bucket, "prefix", s3Prefix)
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config for S3: %w", err)
		}
		uploader := core.NewS3Uploader(cfg, s3Bucket, s3Prefix)
		if err := uploader.UploadDirectory(outputDir); err != nil {
			return fmt.Errorf("failed to upload output to s3: %w", err)
		}
		slog.Info("Successfully uploaded to S3")
	}

	return nil
}
```

- [ ] **Step 2: Replace `main.go` body with the dispatcher**

Replace the contents of `cmd/fibr-gen/main.go` with:

```go
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func run(output io.Writer, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "validate":
			return runValidate(output, args[1:])
		case "generate":
			return runGenerate(output, args[1:])
		case "help", "--help", "-h":
			fmt.Fprintf(output, "Usage: fibr-gen [generate|validate] [flags]\n")
			fmt.Fprintf(output, "  generate  Generate an Excel report (default)\n")
			fmt.Fprintf(output, "  validate  Validate config against template without generating\n")
			return nil
		}
	}
	// Default: generate (backward compat — args may be flags like -config ...)
	return runGenerate(output, args)
}
```

- [ ] **Step 3: Create a stub for `cmd_validate.go` so the package compiles**

Create `cmd/fibr-gen/cmd_validate.go` with:

```go
package main

import (
	"fmt"
	"io"
)

func runValidate(output io.Writer, args []string) error {
	fmt.Fprintln(output, "validate: not yet implemented")
	return nil
}
```

- [ ] **Step 4: Verify the package compiles and existing test passes**

Run:
```bash
go build ./cmd/fibr-gen/
go test ./cmd/fibr-gen/ -run TestRun -v
```

Expected: `TestRun` passes (backward-compat path still routes to `runGenerate`).

- [ ] **Step 5: Commit**

```bash
git add cmd/fibr-gen/main.go cmd/fibr-gen/cmd_generate.go cmd/fibr-gen/cmd_validate.go
git commit -m "refactor: split CLI into generate/validate subcommands"
```

---

## Task 2: Raw Config Loader

`config.LoadConfigBundle` runs strict validation and will fail-fast on any missing field. The `validate` command needs to load the config leniently so it can report all issues itself.

**Files:**
- Modify: `config/loader.go`
- Modify: `config/loader_test.go`

- [ ] **Step 1: Write a failing test for `LoadConfigBundleRaw`**

Add to `config/loader_test.go`:

```go
func TestLoadConfigBundleRaw_SkipsValidation(t *testing.T) {
	dir := t.TempDir()
	// Config with an intentionally missing DataSource DSN (strict loader would reject this)
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
```

Also add the missing imports to the test file (`"os"`, `"path/filepath"`) if not already present.

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./config/ -run TestLoadConfigBundleRaw_SkipsValidation -v
```

Expected: FAIL — `undefined: LoadConfigBundleRaw`

- [ ] **Step 3: Implement `LoadConfigBundleRaw` in `config/loader.go`**

Add after the existing `LoadConfigBundle` function:

```go
// LoadConfigBundleRaw parses a config bundle YAML without running any validation.
// Use this in the validate command to collect all issues rather than fail-fast.
func LoadConfigBundleRaw(path string) (*WorkbookConfig, map[string]*DataViewConfig, map[string]*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read config bundle: %w", err)
	}

	var bundle BundleConfig
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse config bundle: %w", err)
	}

	if bundle.Workbook == nil {
		return nil, nil, nil, fmt.Errorf("config bundle missing workbook")
	}

	views := make(map[string]*DataViewConfig)
	for _, view := range bundle.DataViews {
		if view != nil && view.Name != "" {
			views[view.Name] = view
		}
	}

	dataSources := make(map[string]*DataSourceConfig)
	for _, source := range bundle.DataSources {
		if source != nil && source.Name != "" {
			dataSources[source.Name] = source
		}
	}

	return bundle.Workbook, views, dataSources, nil
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./config/ -run TestLoadConfigBundleRaw_SkipsValidation -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config/loader.go config/loader_test.go
git commit -m "feat: add LoadConfigBundleRaw for lenient config parsing"
```

---

## Task 3: TemplateValidator — Sheet and Range Checks

**Files:**
- Create: `core/validate.go`
- Create: `core/validate_test.go`

- [ ] **Step 1: Write failing tests for sheet-not-found and invalid range**

Create `core/validate_test.go`:

```go
package core

import (
	"fibr-gen/config"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// makeTemplateFile creates a minimal xlsx in dir with the given sheet name and cell value.
func makeTemplateFile(t *testing.T, dir, filename, sheetName, cell, value string) string {
	t.Helper()
	f := excelize.NewFile()
	// excelize creates "Sheet1" by default; rename or add as needed
	if sheetName != "Sheet1" {
		f.NewSheet(sheetName)
		f.DeleteSheet("Sheet1")
	}
	if cell != "" {
		f.SetCellValue(sheetName, cell, value)
	}
	path := filepath.Join(dir, filename)
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("makeTemplateFile: %v", err)
	}
	return path
}

func TestTemplateValidator_SheetNotFound(t *testing.T) {
	dir := t.TempDir()
	makeTemplateFile(t, dir, "tpl.xlsx", "Sheet1", "", "")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{Name: "MissingSheet", Blocks: []config.BlockConfig{}},
		},
	}

	tv := NewTemplateValidator(wb, nil, dir)
	issues := tv.Validate()

	if len(issues) == 0 {
		t.Fatal("expected issues, got none")
	}
	found := false
	for _, iss := range issues {
		if iss.Level == IssueLevelError && iss.Category == "template" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ERROR [template] issue, got: %v", issues)
	}
}

func TestTemplateValidator_InvalidRange(t *testing.T) {
	dir := t.TempDir()
	makeTemplateFile(t, dir, "tpl.xlsx", "Sheet1", "", "")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:  "block1",
						Type:  config.BlockTypeValue,
						Range: config.CellRange{Ref: "NOTARANGE"},
					},
				},
			},
		},
	}

	tv := NewTemplateValidator(wb, nil, dir)
	issues := tv.Validate()

	found := false
	for _, iss := range issues {
		if iss.Level == IssueLevelError && iss.Category == "template" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ERROR [template] for invalid range, got: %v", issues)
	}
}

func TestTemplateValidator_ValidConfig_NoIssues(t *testing.T) {
	dir := t.TempDir()
	makeTemplateFile(t, dir, "tpl.xlsx", "Sales", "A1", "hello")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name: "Sales",
				Blocks: []config.BlockConfig{
					{
						Name:  "b1",
						Type:  config.BlockTypeValue,
						Range: config.CellRange{Ref: "A1:B2"},
					},
				},
			},
		},
	}

	tv := NewTemplateValidator(wb, map[string]*config.DataViewConfig{}, dir)
	issues := tv.Validate()

	for _, iss := range issues {
		if iss.Level == IssueLevelError {
			t.Errorf("unexpected ERROR: %s", iss.Message)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./core/ -run "TestTemplateValidator" -v
```

Expected: FAIL — `undefined: NewTemplateValidator`, `undefined: IssueLevelError`

- [ ] **Step 3: Implement `core/validate.go` with sheet and range checks**

Create `core/validate.go`:

```go
package core

import (
	"fibr-gen/config"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// IssueLevel is the severity of a ValidationIssue.
type IssueLevel string

const (
	IssueLevelError IssueLevel = "ERROR"
	IssueLevelWarn  IssueLevel = "WARN"
)

// ValidationIssue is a single finding from template validation.
type ValidationIssue struct {
	Level    IssueLevel
	Category string // "config" or "template"
	Message  string
}

func (i ValidationIssue) String() string {
	return fmt.Sprintf("%-6s [%s] %s", string(i.Level), i.Category, i.Message)
}

// TemplateValidator cross-checks a WorkbookConfig against its Excel template.
type TemplateValidator struct {
	workbook    *config.WorkbookConfig
	views       map[string]*config.DataViewConfig
	templateDir string
}

// NewTemplateValidator creates a TemplateValidator. views may be nil.
func NewTemplateValidator(wb *config.WorkbookConfig, views map[string]*config.DataViewConfig, templateDir string) *TemplateValidator {
	if views == nil {
		views = make(map[string]*config.DataViewConfig)
	}
	return &TemplateValidator{workbook: wb, views: views, templateDir: templateDir}
}

// Validate runs all checks and returns every issue found (no fail-fast).
func (v *TemplateValidator) Validate() []ValidationIssue {
	var issues []ValidationIssue

	templatePath := filepath.Join(v.templateDir, v.workbook.Template)
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return append(issues, ValidationIssue{
			Level:    IssueLevelError,
			Category: "template",
			Message:  fmt.Sprintf("cannot open template %q: %v", v.workbook.Template, err),
		})
	}
	defer f.Close()

	sheetSet := make(map[string]bool)
	for _, s := range f.GetSheetList() {
		sheetSet[s] = true
	}

	// Build label index: label name → list of view names that define it
	labelToViews := make(map[string][]string)
	for viewName, dv := range v.views {
		for _, l := range dv.Labels {
			labelToViews[l.Name] = append(labelToViews[l.Name], viewName)
		}
	}

	usedLabels := make(map[string]bool)

	for _, sheet := range v.workbook.Sheets {
		if !sheetSet[sheet.Name] {
			issues = append(issues, ValidationIssue{
				Level:    IssueLevelError,
				Category: "template",
				Message:  fmt.Sprintf("sheet %q not found in template", sheet.Name),
			})
			continue
		}
		for i := range sheet.Blocks {
			issues = append(issues, v.validateBlock(f, sheet.Name, &sheet.Blocks[i], labelToViews, usedLabels)...)
		}
	}

	// Warn about defined labels that never appear in the template
	for viewName, dv := range v.views {
		for _, l := range dv.Labels {
			if !usedLabels[l.Name] {
				issues = append(issues, ValidationIssue{
					Level:    IssueLevelWarn,
					Category: "config",
					Message:  fmt.Sprintf("DataView %q label %q unused in template", viewName, l.Name),
				})
			}
		}
	}

	return issues
}

func (v *TemplateValidator) validateBlock(f *excelize.File, sheetName string, block *config.BlockConfig, labelToViews map[string][]string, usedLabels map[string]bool) []ValidationIssue {
	var issues []ValidationIssue

	ref := block.Range.Ref
	if ref == "" {
		return issues
	}

	if err := validateRangeSyntax(ref); err != nil {
		issues = append(issues, ValidationIssue{
			Level:    IssueLevelError,
			Category: "template",
			Message:  fmt.Sprintf("sheet %q block %q: range %q is invalid: %v", sheetName, block.Name, ref, err),
		})
		// Cannot scan cells if range is unparseable; still recurse into sub-blocks
	} else {
		issues = append(issues, v.scanPlaceholders(f, sheetName, block.Name, ref, labelToViews, usedLabels)...)
	}

	for i := range block.SubBlocks {
		issues = append(issues, v.validateBlock(f, sheetName, &block.SubBlocks[i], labelToViews, usedLabels)...)
	}

	return issues
}

// validateRangeSyntax checks that a ref like "A1" or "A1:G33" is syntactically valid.
func validateRangeSyntax(ref string) error {
	parts := strings.SplitN(ref, ":", 2)
	if _, _, err := excelize.CellNameToCoordinates(parts[0]); err != nil {
		return fmt.Errorf("invalid cell %q: %v", parts[0], err)
	}
	if len(parts) == 2 {
		if _, _, err := excelize.CellNameToCoordinates(parts[1]); err != nil {
			return fmt.Errorf("invalid cell %q: %v", parts[1], err)
		}
	}
	return nil
}

// cellsInRange returns non-empty cell values for all cells in the given range ref.
func cellsInRange(f *excelize.File, sheetName, ref string) map[string]string {
	result := make(map[string]string)
	parts := strings.SplitN(ref, ":", 2)
	startCol, startRow, _ := excelize.CellNameToCoordinates(parts[0])
	endCol, endRow := startCol, startRow
	if len(parts) == 2 {
		endCol, endRow, _ = excelize.CellNameToCoordinates(parts[1])
	}
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			name, _ := excelize.CoordinatesToCellName(col, row)
			val, _ := f.GetCellValue(sheetName, name)
			if val != "" {
				result[name] = val
			}
		}
	}
	return result
}

// scanPlaceholders is added in Task 4. Stub here returns no issues.
func (v *TemplateValidator) scanPlaceholders(f *excelize.File, sheetName, blockName, ref string, labelToViews map[string][]string, usedLabels map[string]bool) []ValidationIssue {
	return nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./core/ -run "TestTemplateValidator" -v
```

Expected: all three `TestTemplateValidator_*` tests PASS.

- [ ] **Step 5: Commit**

```bash
git add core/validate.go core/validate_test.go
git commit -m "feat: add TemplateValidator with sheet and range checks"
```

---

## Task 4: TemplateValidator — Placeholder Coverage

**Files:**
- Modify: `core/validate.go` (replace `scanPlaceholders` stub)
- Modify: `core/validate_test.go` (add two new test cases)

Template cells use `{label_name}` syntax (see `core/generator.go:fillTemplate`). Scan each cell in a block's range and report unknown labels as WARN.

- [ ] **Step 1: Write failing tests for placeholder checks**

Add to `core/validate_test.go`:

```go
func TestTemplateValidator_UnknownPlaceholder_Warns(t *testing.T) {
	dir := t.TempDir()
	// Template has {emp_name} in A1 but no DataView defines that label
	makeTemplateFile(t, dir, "tpl.xlsx", "Sheet1", "A1", "{emp_name}")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:  "b1",
						Type:  config.BlockTypeValue,
						Range: config.CellRange{Ref: "A1"},
					},
				},
			},
		},
	}

	tv := NewTemplateValidator(wb, map[string]*config.DataViewConfig{}, dir)
	issues := tv.Validate()

	found := false
	for _, iss := range issues {
		if iss.Level == IssueLevelWarn && iss.Category == "template" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected WARN [template] for unknown placeholder, got: %v", issues)
	}
}

func TestTemplateValidator_UnusedLabel_Warns(t *testing.T) {
	dir := t.TempDir()
	// Template has no placeholders, but DataView defines a label
	makeTemplateFile(t, dir, "tpl.xlsx", "Sheet1", "A1", "static value")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:  "b1",
						Type:  config.BlockTypeValue,
						Range: config.CellRange{Ref: "A1"},
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"view1": {
			Name: "view1",
			Labels: []config.LabelConfig{
				{Name: "emp_name", Column: "name"},
			},
		},
	}

	tv := NewTemplateValidator(wb, views, dir)
	issues := tv.Validate()

	found := false
	for _, iss := range issues {
		if iss.Level == IssueLevelWarn && iss.Category == "config" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected WARN [config] for unused label, got: %v", issues)
	}
}

func TestTemplateValidator_KnownPlaceholder_NoWarn(t *testing.T) {
	dir := t.TempDir()
	makeTemplateFile(t, dir, "tpl.xlsx", "Sheet1", "A1", "{emp_name}")

	wb := &config.WorkbookConfig{
		Name:     "R",
		Template: "tpl.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:  "b1",
						Type:  config.BlockTypeValue,
						Range: config.CellRange{Ref: "A1"},
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"view1": {
			Name: "view1",
			Labels: []config.LabelConfig{
				{Name: "emp_name", Column: "name"},
			},
		},
	}

	tv := NewTemplateValidator(wb, views, dir)
	issues := tv.Validate()

	for _, iss := range issues {
		if iss.Level == IssueLevelWarn {
			t.Errorf("unexpected WARN: %s", iss.Message)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm the new ones fail**

```bash
go test ./core/ -run "TestTemplateValidator" -v
```

Expected: `TestTemplateValidator_UnknownPlaceholder_Warns` FAIL, `TestTemplateValidator_UnusedLabel_Warns` FAIL, `TestTemplateValidator_KnownPlaceholder_NoWarn` PASS (stub returns no issues, no WARNs).

- [ ] **Step 3: Implement `scanPlaceholders` in `core/validate.go`**

Add this import to `core/validate.go`:

```go
"regexp"
```

Replace the stub `scanPlaceholders` function with:

```go
var placeholderRe = regexp.MustCompile(`\{([^{}]+)\}`)

func (v *TemplateValidator) scanPlaceholders(f *excelize.File, sheetName, blockName, ref string, labelToViews map[string][]string, usedLabels map[string]bool) []ValidationIssue {
	var issues []ValidationIssue
	for cellRef, cellValue := range cellsInRange(f, sheetName, ref) {
		idxPairs := placeholderRe.FindAllStringSubmatchIndex(cellValue, -1)
		for _, m := range idxPairs {
			// Skip ${param} style (parameter substitutions, not data labels)
			if m[0] > 0 && cellValue[m[0]-1] == '$' {
				continue
			}
			labelName := cellValue[m[2]:m[3]]
			if _, known := labelToViews[labelName]; known {
				usedLabels[labelName] = true
			} else {
				issues = append(issues, ValidationIssue{
					Level:    IssueLevelWarn,
					Category: "template",
					Message:  fmt.Sprintf("sheet %q cell %s: placeholder {%s} not found in any DataView", sheetName, cellRef, labelName),
				})
			}
		}
	}
	return issues
}
```

- [ ] **Step 4: Run all TemplateValidator tests**

```bash
go test ./core/ -run "TestTemplateValidator" -v
```

Expected: all six `TestTemplateValidator_*` tests PASS.

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add core/validate.go core/validate_test.go
git commit -m "feat: add placeholder coverage checks to TemplateValidator"
```

---

## Task 5: `runValidate` Command

**Files:**
- Modify: `cmd/fibr-gen/cmd_validate.go` (replace stub)
- Modify: `cmd/fibr-gen/main_test.go` (add integration tests)

- [ ] **Step 1: Write failing integration tests**

Add to `cmd/fibr-gen/main_test.go`:

```go
func TestValidate_CleanConfig(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Template with Sheet1 and no placeholders
	templatePath := filepath.Join(templateDir, "template.xlsx")
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "hello")
	if err := f.SaveAs(templatePath); err != nil {
		t.Fatalf("save template: %v", err)
	}

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

	// Template only has Sheet1, but config references MissingSheet
	templatePath := filepath.Join(templateDir, "template.xlsx")
	f := excelize.NewFile()
	if err := f.SaveAs(templatePath); err != nil {
		t.Fatalf("save template: %v", err)
	}

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
```

Also add `"strings"` to the imports in `main_test.go` if not already present.

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./cmd/fibr-gen/ -run "TestValidate" -v
```

Expected: `TestValidate_CleanConfig` FAIL (stub prints "not yet implemented"), `TestValidate_MissingSheet_ReportsError` FAIL.

- [ ] **Step 3: Implement `runValidate` in `cmd/fibr-gen/cmd_validate.go`**

Replace the stub with:

```go
package main

import (
	"fibr-gen/config"
	"fibr-gen/core"
	"flag"
	"fmt"
	"io"
	"log/slog"
)

func runValidate(output io.Writer, args []string) error {
	flags := flag.NewFlagSet("fibr-gen validate", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.Usage = func() {
		fmt.Fprintf(output, "Usage: fibr-gen validate [flags]\n\nFlags:\n")
		flags.PrintDefaults()
	}

	var (
		configFile     string
		dataSourceFile string
		templateDir    string
	)

	flags.StringVar(&configFile, "config", "./test/config.yaml", "Path to configuration bundle")
	flags.StringVar(&configFile, "c", "./test/config.yaml", "Path to configuration bundle (short)")
	flags.StringVar(&dataSourceFile, "datasources", "", "Path to data source bundle (optional)")
	flags.StringVar(&templateDir, "templates", "./test/templates", "Template group directory")
	flags.StringVar(&templateDir, "t", "./test/templates", "Template group directory (short)")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("Loading config", "file", configFile)
	wbConf, views, dataSources, err := config.LoadConfigBundleRaw(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if dataSourceFile != "" {
		extra, err := config.LoadDataSourcesBundle(dataSourceFile)
		if err != nil {
			return fmt.Errorf("failed to load data sources: %w", err)
		}
		for k, v := range extra {
			dataSources[k] = v
		}
	}

	slog.Info("Loading template", "dir", templateDir, "file", wbConf.Template)

	tv := core.NewTemplateValidator(wbConf, views, templateDir)
	issues := tv.Validate()

	// Check DataSource completeness (offline — no connection attempt)
	for _, ds := range dataSources {
		if ds.DSN == "" {
			issues = append(issues, core.ValidationIssue{
				Level:    core.IssueLevelWarn,
				Category: "config",
				Message:  fmt.Sprintf("DataSource %q has empty DSN", ds.Name),
			})
		}
	}

	errorCount := 0
	warnCount := 0
	for _, iss := range issues {
		fmt.Fprintln(output, iss.String())
		switch iss.Level {
		case core.IssueLevelError:
			errorCount++
		case core.IssueLevelWarn:
			warnCount++
		}
	}

	fmt.Fprintf(output, "INFO   Validation complete: %d error(s), %d warning(s)\n", errorCount, warnCount)

	if errorCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errorCount)
	}
	return nil
}
```

- [ ] **Step 4: Run integration tests**

```bash
go test ./cmd/fibr-gen/ -run "TestValidate" -v
```

Expected: both `TestValidate_*` tests PASS.

- [ ] **Step 5: Run the full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Smoke test the binary manually**

```bash
go build -o /tmp/fibr-gen ./cmd/fibr-gen/
/tmp/fibr-gen validate -c test/workbooks/demo_report.yaml -t test/templates
```

Expected: output lines with INFO/WARN/ERROR, ending with `Validation complete: N error(s), N warning(s)`.

- [ ] **Step 7: Commit**

```bash
git add cmd/fibr-gen/cmd_validate.go cmd/fibr-gen/main_test.go
git commit -m "feat: implement fibr-gen validate subcommand"
```
