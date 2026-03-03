# 配置指南

本指南将介绍如何配置 **fibr-gen** 以生成报表。配置主要涉及 **数据视图 (Data View)**（数据源）和 **工作簿 (Workbook)**（报表布局）的定义。

## 核心概念

### 1. 数据视图 (Data View)
数据视图将您的数据源（例如 SQL 查询或内存对象）抽象为一组 **标签 (Labels)**。
- **标签 (Label)**：数据列（如数据库中的 `user_name`）与模板变量（如 `{name}`）之间的映射。
- 在 Excel 模板中，使用 `{name}` 格式来引用此标签。

### 2. 工作簿 (Workbook) 与 工作表 (Sheet)
- **工作簿**：对应一个 Excel 文件。
- **工作表**：对应 Excel 文件中的一个 Sheet。
  - **静态工作表**：模板中定义的标准 Sheet。
  - **动态工作表**：根据数据自动复制的 Sheet（例如，每个员工生成一个 Sheet）。

### 3. 数据块 (Block)
数据块定义了 Excel 模板中的一个矩形区域，以及如何将数据填充到该区域中。
- **值块 (ValueBlock, 类型: value)**：按顺序（垂直或水平）填充数据。
- **矩阵块 (MatrixBlock, 类型: matrix)**：用于复杂布局的容器。通常包含 **表头块 (HeaderBlock)** 来定义扩展方向。
- **表头块 (HeaderBlock, 类型: header)**：定义矩阵块的表头/轴（例如，顶部的月份列表，左侧的产品列表）。

---

## 配置结构

配置通常使用 JSON 或 YAML 定义。

### 数据视图配置 (`DataViewConfig`)

```yaml
id: "v_employee"
name: "员工视图"
dataSource: "db_main"
sql: "SELECT * FROM employees"
labels:
  - name: "emp_name"   # 在 Excel 中使用 {emp_name}
    column: "NAME"     # 数据库列名
  - name: "emp_dept"
    column: "DEPT"
```

### 工作簿配置 (`WorkbookConfig`)

```yaml
id: "wb_report_01"
name: "月度报表"
template: "template.xlsx"
outputDir: "output/"
sheets:
  - name: "Sheet1"
    blocks:
      - name: "EmployeeList"
        type: "value"         # 值块
        range: { ref: "A2:B2" }
        dataView: "v_employee"
        direction: "vertical" # 向下扩展
```

---

## 数据块类型详解

### 1. 值块 (Value Block, `value`)
最简单的块类型。它获取一个范围（例如单行 `A2:B2`），并为数据中的每条记录重复该范围。

**示例：**
模板：
```
| 姓名       | 部门       |
| {emp_name} | {emp_dept} |
```

结果（3条记录）：
```
| 姓名       | 部门       |
| Alice      | HR         |
| Bob        | IT         |
| Charlie    | IT         |
```

### 2. 矩阵块 (`matrix`) 与 表头块 (`header`)
用于矩阵/交叉表报表。

**结构：**
- **矩阵块** (容器，覆盖整个区域)
  - **表头块** (垂直)：例如，员工列表。
  - **表头块** (水平)：例如，月份列表。
  - **值块** (数据)：交叉点（例如，销售额）。

**配置示例：**

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

## 动态工作表 (Dynamic Sheets)

要从单个模板 Sheet 生成多个 Sheet：

1. 在 `SheetConfig` 中设置 `dynamic: true`。
2. 指定 `dataView` 以提供项目列表（例如，部门列表）。
3. 指定 `paramLabel` 以标识键（例如，`dept_id`）。

生成器将执行以下操作：
1. 从指定视图获取数据。
2. 为每条记录复制模板 Sheet。
3. 重命名 Sheet（通常使用参数值）。
4. 将参数（例如 `dept_id=D01`）注入上下文，以便内部的块可以据此过滤数据。

---

## 架构概览

### 系统组成

fibr-gen 是一个 Excel 报表自动生成引擎：读取 Excel 模板 + YAML 配置 + 数据源，自动填充数据并输出成品报表。

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  YAML 配置    │     │  Excel 模板   │     │   数据源      │
│  (Workbook +  │     │  (.xlsx)      │     │  (SQL/CSV/   │
│   DataView)   │     │              │     │   DynamoDB)   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────┬───────┘────────────────────┘
                    ▼
            ┌──────────────┐
            │  Generator   │
            │  (核心引擎)    │
            └──────┬───────┘
                   ▼
            ┌──────────────┐     ┌──────────────┐
            │  输出 Excel   │────▶│  S3 上传      │
            │  (.xlsx)      │     │  (可选)       │
            └──────────────┘     └──────────────┘
```

### 关键源文件

| 文件 | 职责 |
|------|------|
| `config/types.go` | 所有配置结构体定义（Block、Sheet、DataView 等） |
| `config/loader.go` | 从 YAML 加载配置（支持单文件 bundle 或目录扫描） |
| `config/provider.go` | 配置注册中心接口（运行时按名查找 DataView） |
| `config/validator.go` | 配置校验（检查数据视图引用、块结构合法性等） |
| `core/generator.go` | **核心引擎**：模板处理、行列插入、数据填充 |
| `core/context.go` | 运行时上下文，管理参数传递和数据获取 |
| `core/dataview.go` | DataView 抽象层（过滤、去重、深拷贝） |
| `core/dynamic_date.go` | 动态日期解析（如 `$date:day:day:-1` = 昨天） |
| `core/excel_adapter.go` | Excel 操作接口抽象（基于 excelize 库） |
| `core/sql_fetcher.go` | SQL 数据源（MySQL/PostgreSQL） |
| `core/csv_fetcher.go` | CSV 文件数据源 |
| `core/dynamodb_fetcher.go` | AWS DynamoDB 数据源 |
| `core/s3_uploader.go` | 生成后上传至 AWS S3 |
| `cmd/fibr-gen/main.go` | CLI 入口 |
| `cmd/lambda/main.go` | AWS Lambda 入口 |

---

## 生成流程

```
1. 加载 YAML 配置
2. 初始化 GenerationContext（全局参数、数据源、配置注册中心）
3. 打开 Excel 模板文件
4. 遍历每个 Sheet：
   ├─ 动态 Sheet？→ 按参数标签的去重值克隆多个 Sheet
   └─ 遍历每个 Block：
       ├─ Value Block  → 取数据 → 按行/列扩展 → 填充单元格
       ├─ Header Block → 取数据 → 去重 → 按方向扩展
       └─ Matrix Block → 识别双轴 → 插行/列 → 遍历网格 → 填交叉数据
5. 保存输出 Excel 文件
6.（可选）上传至 S3
```

---

## 参数过滤机制

这是理解矩阵数据填充的关键。

### 工作原理

```
全局参数 → Fetcher.Fetch() 取全量数据 → 缓存到 DataView
  → 每个 Block 调用 GetBlockDataWithParams(block, params)
    → DataView.Copy()       创建深拷贝，避免污染缓存
    → DataView.Filter(params) 按参数过滤
    → Header Block?  → distinctData() 去重
    → Value Block?   → 直接返回过滤后的行
```

### 矩阵中的参数传递

在矩阵遍历中，参数会**逐层叠加**。例如一个销售报表矩阵：

```
外层矩阵：行轴=地区，列轴=产品
内层矩阵：行轴=指标，列轴=时段
```

当处理 "地区=华东, 产品=Alpha" 这个交叉格时：
1. 外层矩阵将 `{region: "华东", product: "Alpha"}` 注入 `cellParams`
2. 内层矩阵接收这些参数
3. 内层的数据块查询时自动带上 `region=华东 AND product=Alpha` 过滤
4. 结果：每个交叉格只显示对应的数据

### 单数据视图的妙用

所有块可以引用**同一个 DataView**。系统会自动处理：
- **Header Block**：自动对 `labelVariable` 指定的列做去重（distinct）
- **Value Block**：返回与当前参数匹配的所有行

这意味着一个包含 `region, product, metric, period, amount` 五列的视图，可以同时服务于外层行轴（按 region 去重）、外层列轴（按 product 去重）和内层数据块（按参数过滤后取 amount）。

---

## 矩阵块的嵌套（递归矩阵）

矩阵块支持嵌套：外层矩阵的数据区域内可以放置内层矩阵。

### 约束

- 内层矩阵**不允许扩展行列**（即每个轴只能返回 1 条数据）
- 内层矩阵的数据通过外层传入的参数自动过滤

### 示例布局

```
      A       B       C       D
 1                   {product}          ← 外层水平轴（C1:D2 合并单元格）
 2                   (合并区域)
 3   {region}                 {period}  ← 外层垂直轴（A3:B4 合并单元格）| 内层水平轴
 4   (合并区域)       {metric} {amount}  ← 内层垂直轴 | 内层数据
```

展开后（2个地区 × 2个产品）：

```
      A       B       C        D        E        F
 1                   Alpha             Beta
 2                   (合并)             (合并)
 3   East                     Q1                Q1
 4   (合并)          Revenue   100     Revenue   200
 5   West                     Q1                Q1
 6   (合并)          Revenue   300     Revenue   400
```

---

## 动态日期

支持相对日期表达式，格式为 `$date:输出格式:偏移单位:偏移量`。

| 表达式 | 含义 | 示例输出 |
|--------|------|----------|
| `$date:day:day:0` | 今天 | `2025-03-01` |
| `$date:day:day:-1` | 昨天 | `2025-02-28` |
| `$date:month:month:0` | 本月 | `2025-03` |
| `$date:year:year:-1` | 去年 | `2024` |

可在工作簿的 `parameters` 中使用，也可作为 `archiveRule` 进行时间归档过滤。

---

## 模板缓存机制

`generator.go` 中的 `captureTemplate()` / `fillTemplate()` 实现了高效的模板复用：

1. **captureTemplate()**：首次读取模板区域，缓存所有单元格的值、样式 ID、合并单元格信息
2. **fillTemplate()**：将缓存内容写入新位置，替换 `{label}` 占位符为实际数据

这避免了在矩阵展开时反复读取模板，显著提升大规模报表的生成性能。

---

## 运行方式

### CLI 模式

```bash
fibr-gen \
  -config config.yaml \
  -templates ./templates \
  -output ./output \
  -fetcher mysql \
  -db-dsn "user:pass@tcp(localhost)/db"
```

### AWS Lambda 模式

通过 `cmd/lambda/main.go` 部署为 Lambda 函数，适合定时触发或事件驱动的报表生成。

### 可选：S3 上传

```bash
fibr-gen ... -s3-bucket my-bucket -s3-prefix reports/
```

生成完成后自动将报表上传至指定 S3 路径。
