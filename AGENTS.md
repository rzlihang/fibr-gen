# fibr-gen

Excel report generation engine in Go. Reads an Excel template + YAML config + data source, fills data automatically, and outputs finished reports.

## Build & Test

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./core/ -v

# Run a specific test
go test ./core/ -run TestMatrixBlock_NestedMatrix -v

# Build CLI binary
go build -o fibr-gen ./cmd/fibr-gen

# Build Lambda binary
GOOS=linux GOARCH=amd64 go build -o lambda ./cmd/lambda
```

## Project Structure

```
config/       Configuration structs, YAML loader, provider, validator
core/         Generation engine, data fetchers, Excel adapter
cmd/fibr-gen/ CLI entry point
cmd/lambda/   AWS Lambda entry point
docs/         Documentation (EN/ZH/JA)
test/         Sample templates and config files
```

## Key Concepts

### Block Types (config/types.go)
- **`value`** — Fills cells with data rows, expands vertically or horizontally
- **`header`** — Matrix axis (row or column header), deduplicated
- **`matrix`** — Container with 2 header sub-blocks + value/nested-matrix sub-blocks

### DataView
Maps database columns to template label names. All blocks reference a DataView by name. Header blocks auto-dedup; value blocks return filtered rows.

### Template Placeholders
Excel cells use `{label_name}` syntax. E.g. a cell containing `{emp_name}` is replaced with data from the label `emp_name` in the bound DataView.

## Architecture Notes

- **`core/generator.go`** is the main engine. `processMatrixBlock()` is the most complex function — it handles 2D expansion, row/col insertion, and recursive nested matrix support.
- **`core/context.go`** manages runtime state. `GetBlockDataWithParams()` is the central data-fetch path — it deep-copies the DataView, filters by params, then deduplicates (for headers) or returns rows directly (for values).
- Parameters are layered: outer matrix injects `{row_key, col_key}` into `cellParams` before processing each intersection, so inner blocks automatically receive them for filtering.
- **Template caching**: `captureTemplate()` reads a template region once (values, style IDs, merged cells); `fillTemplate()` writes it to new positions replacing placeholders. This avoids repeated reads during matrix expansion.

## Adding a New Data Fetcher

1. Implement the `DataFetcher` interface in `core/`:
   ```go
   type DataFetcher interface {
       Fetch(viewName string, params map[string]string) ([]map[string]interface{}, error)
   }
   ```
2. Wire it into the CLI flags in `cmd/fibr-gen/main.go`.

## Test Output

Generated test Excel files are saved to `test_output/` (gitignored). Pass `-v` to see file paths logged during tests.
