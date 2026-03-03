# Configuration Guide

This guide explains how to configure **fibr-gen** to generate reports. The configuration involves defining **Data Views** (data sources) and **Workbooks** (report layout).

## Core Concepts

### 1. Data View
A Data View abstracts your data source (e.g., a SQL query or a memory object) into a set of **Labels**.
- **Label**: A mapping between a data column (e.g., `user_name`) and a template variable (e.g., `name`).
- In the Excel template, you use `{name}` to refer to this label.

### 2. Workbook & Sheet
- **Workbook**: Corresponds to an Excel file.
- **Sheet**: Corresponds to a sheet within the Excel file.
  - **Static Sheet**: A standard sheet defined in the template.
  - **Dynamic Sheet**: A sheet that is replicated based on data (e.g., one sheet per employee).

### 3. Block
A Block defines a rectangular region in the Excel template and how data should be filled into it.
- **ValueBlock (Type: value)**: Fills data sequentially (vertically or horizontally).
- **MatrixBlock (Type: matrix)**: A container for complex layouts. It usually contains **HeaderBlocks** to define expansion directions.
- **HeaderBlock (Type: header)**: Defines the header/axis for a MatrixBlock (e.g., list of months across the top, list of products down the side).

---

## Configuration Structure

The configuration is typically defined in JSON or YAML.

### Data View Config (`DataViewConfig`)

```yaml
id: "v_employee"
name: "Employee View"
dataSource: "db_main"
sql: "SELECT * FROM employees"
labels:
  - name: "emp_name"   # Use {emp_name} in Excel
    column: "NAME"     # Column in DB
  - name: "emp_dept"
    column: "DEPT"
```

### Workbook Config (`WorkbookConfig`)

```yaml
id: "wb_report_01"
name: "Monthly Report"
template: "template.xlsx"
outputDir: "output/"
sheets:
  - name: "Sheet1"
    blocks:
      - name: "EmployeeList"
        type: "value"         # ValueBlock
        range: { ref: "A2:B2" }
        dataView: "v_employee"
        direction: "vertical" # Expand downwards
```

---

## Block Types Explained

### 1. Value Block (`value`)
The simplest block. It takes a range (e.g., a single row `A2:B2`) and repeats it for every record in the data.

**Example:**
Template:
```
| Name       | Dept       |
| {emp_name} | {emp_dept} |
```

Result (3 records):
```
| Name       | Dept       |
| Alice      | HR         |
| Bob        | IT         |
| Charlie    | IT         |
```

### 2. Matrix Block (`matrix`) & Header Block (`header`)
Used for Matrix/Cross-tab reports.

**Structure:**
- **MatrixBlock** (Container, covers the whole area)
  - **HeaderBlock** (Vertical): e.g., List of Employees.
  - **HeaderBlock** (Horizontal): e.g., List of Months.
  - **ValueBlock** (Data): The intersection (e.g., Sales Amount).

**Example Config:**

```yaml
name: "SalesMatrix"
type: "matrix"
range: { ref: "A1:C5" }
subBlocks:
  - name: "MonthAxis"
    type: "header"
    direction: "horizontal"
    range: { ref: "B1:B1" }
    dataView: "v_months"
    labelVariable: "month"

  - name: "EmpAxis"
    type: "header"
    direction: "vertical"
    range: { ref: "A2:A2" }
    dataView: "v_employees"
    labelVariable: "emp_id"

  - name: "SalesData"
    type: "value"
    range: { ref: "B2:B2" }
    dataView: "v_sales"
```

---

## Dynamic Sheets

To generate multiple sheets from a single template sheet:

1. Set `dynamic: true` in `SheetConfig`.
2. Specify `dataView` to provide the list of items (e.g., list of departments).
3. Specify `paramLabel` to identify the key (e.g., `dept_id`).

The generator will:
1. Fetch data from the specified view.
2. For each record, copy the template sheet.
3. Rename the sheet (usually using the parameter value).
4. Inject the parameter (e.g., `dept_id=D01`) into the context so blocks inside can filter data accordingly.

---

## Architecture Overview

### System Components

fibr-gen is an Excel report generation engine: it reads an Excel template + YAML configuration + data sources, fills in data automatically, and outputs finished reports.

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ YAML Config   │     │ Excel Template│     │  Data Source  │
│ (Workbook +   │     │  (.xlsx)      │     │  (SQL/CSV/   │
│  DataView)    │     │              │     │   DynamoDB)   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────┬───────┘────────────────────┘
                    ▼
            ┌──────────────┐
            │  Generator   │
            │ (Core Engine)│
            └──────┬───────┘
                   ▼
            ┌──────────────┐     ┌──────────────┐
            │ Output Excel  │────▶│  S3 Upload   │
            │  (.xlsx)      │     │  (Optional)  │
            └──────────────┘     └──────────────┘
```

### Key Source Files

| File | Responsibility |
|------|----------------|
| `config/types.go` | All configuration structs (Block, Sheet, DataView, etc.) |
| `config/loader.go` | YAML config loading (single bundle or directory scan) |
| `config/provider.go` | Config registry interface (runtime DataView lookup) |
| `config/validator.go` | Config validation (reference checks, block structure) |
| `core/generator.go` | **Core engine**: template processing, row/col insertion, data fill |
| `core/context.go` | Runtime context, parameter management, data fetching |
| `core/dataview.go` | DataView abstraction (filtering, dedup, deep copy) |
| `core/dynamic_date.go` | Dynamic date parsing (e.g., `$date:day:day:-1` = yesterday) |
| `core/excel_adapter.go` | Excel operation interface (based on excelize library) |
| `core/sql_fetcher.go` | SQL data source (MySQL/PostgreSQL) |
| `core/csv_fetcher.go` | CSV file data source |
| `core/dynamodb_fetcher.go` | AWS DynamoDB data source |
| `core/s3_uploader.go` | Post-generation upload to AWS S3 |
| `cmd/fibr-gen/main.go` | CLI entry point |
| `cmd/lambda/main.go` | AWS Lambda entry point |

---

## Generation Flow

```
1. Load YAML configuration
2. Initialize GenerationContext (global params, data source, config registry)
3. Open Excel template file
4. For each Sheet:
   ├─ Dynamic Sheet? → Clone sheet for each distinct value of the param label
   └─ For each Block:
       ├─ Value Block  → Fetch data → Expand rows/cols → Fill cells
       ├─ Header Block → Fetch data → Distinct → Expand in direction
       └─ Matrix Block → Identify axes → Insert rows/cols → Iterate grid → Fill intersections
5. Save output Excel file
6. (Optional) Upload to S3
```

---

## Parameter Filtering Mechanism

This is the key to understanding how matrix data population works.

### How It Works

```
Global params → Fetcher.Fetch() loads full data → Cached in DataView
  → Each Block calls GetBlockDataWithParams(block, params)
    → DataView.Copy()          deep copy to avoid cache pollution
    → DataView.Filter(params)  filter rows by parameters
    → Header Block?  → distinctData() for unique values
    → Value Block?   → return filtered rows directly
```

### Parameter Propagation in Matrices

During matrix iteration, parameters are **layered incrementally**. For example, in a sales report matrix:

```
Outer matrix: row axis = Region, column axis = Product
Inner matrix: row axis = Metric, column axis = Period
```

When processing the intersection "Region=East, Product=Alpha":
1. The outer matrix injects `{region: "East", product: "Alpha"}` into `cellParams`
2. The inner matrix receives these parameters
3. Inner data blocks automatically query with `region=East AND product=Alpha` filter
4. Result: each intersection cell displays only its corresponding data

### Single Data View Pattern

All blocks can reference **the same DataView**. The system handles this automatically:
- **Header Blocks**: Automatically dedup on the column specified by `labelVariable`
- **Value Blocks**: Return all rows matching current parameters

This means a single view with columns like `region, product, metric, period, amount` can simultaneously serve the outer row axis (distinct on region), outer column axis (distinct on product), and inner data blocks (filtered by params to get amount).

---

## Nested Matrices (Recursive Matrix)

Matrix blocks support nesting: an inner matrix can be placed inside the data area of an outer matrix.

### Constraints

- Inner matrices **cannot expand rows or columns** (each axis must return exactly 1 data item)
- Inner matrix data is automatically filtered through parameters passed from the outer matrix

### Example Layout

```
      A       B       C       D
 1                   {product}          ← Outer HAxis (C1:D2 merged cell)
 2                   (merged area)
 3   {region}                 {period}  ← Outer VAxis (A3:B4 merged) | Inner HAxis
 4   (merged)         {metric} {amount} ← Inner VAxis | Inner Values
```

After expansion (2 regions x 2 products):

```
      A       B       C        D        E        F
 1                   Alpha             Beta
 2                   (merged)          (merged)
 3   East                     Q1                Q1
 4   (merged)        Revenue   100     Revenue   200
 5   West                     Q1                Q1
 6   (merged)        Revenue   300     Revenue   400
```

---

## Dynamic Dates

Supports relative date expressions in the format `$date:output_format:offset_unit:offset`.

| Expression | Meaning | Example Output |
|------------|---------|----------------|
| `$date:day:day:0` | Today | `2025-03-01` |
| `$date:day:day:-1` | Yesterday | `2025-02-28` |
| `$date:month:month:0` | This month | `2025-03` |
| `$date:year:year:-1` | Last year | `2024` |

Can be used in workbook `parameters` or as an `archiveRule` for temporal data filtering.

---

## Template Caching

`captureTemplate()` / `fillTemplate()` in `generator.go` implement efficient template reuse:

1. **captureTemplate()**: Reads the template region once, caching all cell values, style IDs, and merged cell info
2. **fillTemplate()**: Writes cached content to a new location, replacing `{label}` placeholders with actual data

This avoids repeated template reads during matrix expansion, significantly improving performance for large reports.

---

## Running fibr-gen

### CLI Mode

```bash
fibr-gen \
  -config config.yaml \
  -templates ./templates \
  -output ./output \
  -fetcher mysql \
  -db-dsn "user:pass@tcp(localhost)/db"
```

### AWS Lambda Mode

Deploy via `cmd/lambda/main.go` as a Lambda function, suitable for scheduled or event-driven report generation.

### Optional: S3 Upload

```bash
fibr-gen ... -s3-bucket my-bucket -s3-prefix reports/
```

Automatically uploads generated reports to the specified S3 path after generation completes.
