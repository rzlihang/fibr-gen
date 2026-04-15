# CLI Configuration Validation — Design Spec

**Date:** 2026-04-15
**Status:** Approved

## Problem

The configuration for fibr-gen (block types, DataViews, cell ranges, placeholders) has a high learning curve. A `validate` subcommand lets users catch config errors early — without needing to understand the full generation pipeline — and provides clear, actionable feedback.

---

## Section 1: Architecture Changes

### Subcommand Structure

Refactor `cmd/fibr-gen/main.go` into three files:

```
cmd/fibr-gen/
  main.go          — entry point, dispatches on first arg
  cmd_generate.go  — existing generate logic (moved)
  cmd_validate.go  — new validate logic
```

Dispatch logic in `main.go`:
- `fibr-gen validate [flags]` → validate flow
- `fibr-gen generate [flags]` → generate flow
- `fibr-gen [flags]` (no subcommand) → generate flow (backward compatible)

### Flags for `validate`

Subset of generate flags — no output, S3, or fetcher flags needed:

```
fibr-gen validate -c config.yaml -t ./templates
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | `./test/config.yaml` | Path to config bundle |
| `--templates` | `-t` | `./test/templates` | Template directory |
| `--datasources` | | | Path to data source bundle (optional) |

---

## Section 2: Validation Logic

### Existing: Structural Validation (`config/validator.go`)

Already validates:
- Required fields (name, template, outputDir, sheet names)
- Block type enum (`value`, `header`, `matrix`)
- Matrix must have both vertical and horizontal header sub-blocks
- DataView/DataSource references exist in the registry

No changes needed here; `validate` will call it as-is.

### New: Template Cross-Validation (`core/validate.go`)

A new `TemplateValidator` struct opens the Excel template via excelize and runs three checks:

**Check 1 — Range validity**

For every `block.Range.Ref` in the config:
- Sheet named in the config exists in the workbook
- Range syntax is parseable (e.g. `A1:G33`)
- Range does not exceed the sheet's actual used bounds

**Check 2 — Placeholder coverage**

Scan every cell in the template for `{label_name}` patterns. For each placeholder found:
- Verify the label exists in the DataView bound to the enclosing block's sheet
- If not found → ERROR

Reverse check: for each label defined in a DataView, if no cell in the template uses it → WARN (unused label)

**Check 3 — DataSource completeness**

For every DataView referenced in the config:
- Its DataSource is defined
- The DataSource has a non-empty DSN

No live connection attempt — validate must work offline.

### Result Collection

All issues are collected before output (no fail-fast). Exit codes:
- `0` — no errors (warnings are ok)
- `1` — one or more errors

---

## Section 3: Output Format & Tests

### Output Format

Plain text, one issue per line, prefixed with severity and category:

```
INFO   Loading config: config.yaml
INFO   Loading template: templates/report.xlsx
ERROR  [config]   block "summary" references unknown DataView "monthly_view"
ERROR  [template] sheet "Sales" block "revenue_matrix": range "ZZ1:ZZ100" is invalid
WARN   [template] sheet "Sales" cell B3: placeholder {emp_dept} not found in any DataView
WARN   [config]   DataView "employee_view" label "emp_code" unused in template
INFO   Validation complete: 2 error(s), 2 warning(s)
```

Levels: `ERROR` (exits 1), `WARN` (exits 0), `INFO` (progress only).

### Tests

Add to `test/` directory:
- `test/configs/invalid_config.yaml` — config with a missing DataView reference and an invalid range
- `test/templates/invalid_template.xlsx` — template with a `{missing_label}` placeholder

Add test cases in `cmd/fibr-gen/main_test.go`:
- `TestValidate_DetectsErrors` — runs `validate` against the bad fixtures, asserts exit code 1 and expected ERROR lines in output
- `TestValidate_CleanConfig` — runs `validate` against existing good fixtures, asserts exit code 0
