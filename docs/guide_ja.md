# 設定ガイド

このガイドでは、**fibr-gen** を使用してレポートを生成するための設定方法について説明します。設定には、**データビュー (Data View)**（データソース）と **ワークブック (Workbook)**（レポートレイアウト）の定義が含まれます。

## コアコンセプト

### 1. データビュー (Data View)
データビューは、データソース（SQLクエリやメモリオブジェクトなど）を **ラベル (Labels)** のセットとして抽象化します。
- **ラベル (Label)**: データ列（例: `user_name`）とテンプレート変数（例: `{name}`）の間のマッピングです。
- Excelテンプレートでは、このラベルを `{name}` という形式で参照します。

### 2. ワークブック (Workbook) と シート (Sheet)
- **ワークブック**: Excelファイルに対応します。
- **シート**: Excelファイル内のシートに対応します。
  - **静的シート**: テンプレートで定義された標準的なシートです。
  - **動的シート**: データに基づいて複製されるシートです（例：従業員ごとに1シート）。

### 3. ブロック (Block)
ブロックは、Excelテンプレート内の矩形領域と、その領域へのデータの入力方法を定義します。
- **値ブロック (ValueBlock, タイプ: value)**: データを順次（垂直または水平）に入力します。
- **マトリックスブロック (MatrixBlock, タイプ: matrix)**: 複雑なレイアウトのためのコンテナです。通常、拡張方向を定義する **ヘッダーブロック (HeaderBlock)** を含みます。
- **ヘッダーブロック (HeaderBlock, タイプ: header)**: マトリックスブロックのヘッダー/軸を定義します（例：上部の月リスト、左側の製品リスト）。

---

## 設定構造

設定は通常、JSONまたはYAMLで定義されます。

### データビュー設定 (`DataViewConfig`)

```yaml
id: "v_employee"
name: "従業員ビュー"
dataSource: "db_main"
sql: "SELECT * FROM employees"
labels:
  - name: "emp_name"   # Excelで {emp_name} として使用
    column: "NAME"     # DBのカラム名
  - name: "emp_dept"
    column: "DEPT"
```

### ワークブック設定 (`WorkbookConfig`)

```yaml
id: "wb_report_01"
name: "月次レポート"
template: "template.xlsx"
outputDir: "output/"
sheets:
  - name: "Sheet1"
    blocks:
      - name: "EmployeeList"
        type: "value"             # タグブロック
        range: { ref: "A2:B2" }
        dataView: "v_employee"
        direction: "vertical" # 下方向に拡張
```

---

## ブロックタイプ解説

### 1. 値ブロック (Value Block, `value`)
最もシンプルなブロックタイプです。範囲（例：1行 `A2:B2`）を取得し、データの各レコードに対してその範囲を繰り返します。

**例:**
テンプレート:
```
| 氏名       | 部署       |
| {emp_name} | {emp_dept} |
```

結果（3レコード）:
```
| 氏名       | 部署       |
| Alice      | 人事部      |
| Bob        | IT部       |
| Charlie    | IT部       |
```

### 2. マトリックスブロック (`matrix`) と ヘッダーブロック (`header`)
マトリックス/クロス集計レポートに使用されます。

**構造:**
- **マトリックスブロック** (コンテナ、全領域をカバー)
  - **ヘッダーブロック** (垂直): 例：従業員リスト。
  - **ヘッダーブロック** (水平): 例：月リスト。
  - **値ブロック** (データ): 交差点（例：売上高）。

**設定例:**

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

## 動的シート (Dynamic Sheets)

単一のテンプレートシートから複数のシートを生成するには：

1. `SheetConfig` で `dynamic: true` を設定します。
2. `dataView` を指定して、アイテムのリスト（例：部署リスト）を提供します。
3. `paramLabel` を指定して、キー（例：`dept_id`）を識別します。

ジェネレーターは以下の操作を実行します：
1. 指定されたビューからデータを取得します。
2. 各レコードに対してテンプレートシートをコピーします。
3. シートの名前を変更します（通常はパラメータ値を使用）。
4. パラメータ（例：`dept_id=D01`）をコンテキストに注入し、内部のブロックがそれに基づいてデータをフィルタリングできるようにします。

---

## アーキテクチャ概要

### システム構成

fibr-gen は Excel レポート自動生成エンジンです。Excel テンプレート + YAML 設定 + データソースを読み込み、データを自動入力して完成レポートを出力します。

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  YAML 設定    │     │ Excel テンプレ │     │ データソース   │
│ (Workbook +   │     │  ート (.xlsx)  │     │ (SQL/CSV/    │
│  DataView)    │     │              │     │  DynamoDB)    │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────┬───────┘────────────────────┘
                    ▼
            ┌──────────────┐
            │  Generator   │
            │ (コアエンジン) │
            └──────┬───────┘
                   ▼
            ┌──────────────┐     ┌──────────────┐
            │  出力 Excel   │────▶│  S3 アップ    │
            │  (.xlsx)      │     │  ロード(任意) │
            └──────────────┘     └──────────────┘
```

### 主要ソースファイル

| ファイル | 役割 |
|----------|------|
| `config/types.go` | すべての設定構造体（Block、Sheet、DataView 等） |
| `config/loader.go` | YAML 設定の読み込み（単一バンドルまたはディレクトリスキャン） |
| `config/provider.go` | 設定レジストリインターフェース（実行時の DataView 検索） |
| `config/validator.go` | 設定バリデーション（参照チェック、ブロック構造の検証） |
| `core/generator.go` | **コアエンジン**：テンプレート処理、行列挿入、データ入力 |
| `core/context.go` | 実行時コンテキスト、パラメータ管理、データ取得 |
| `core/dataview.go` | DataView 抽象層（フィルタリング、重複排除、ディープコピー） |
| `core/dynamic_date.go` | 動的日付解析（例：`$date:day:day:-1` = 昨日） |
| `core/excel_adapter.go` | Excel 操作インターフェース（excelize ライブラリベース） |
| `core/sql_fetcher.go` | SQL データソース（MySQL/PostgreSQL） |
| `core/csv_fetcher.go` | CSV ファイルデータソース |
| `core/dynamodb_fetcher.go` | AWS DynamoDB データソース |
| `core/s3_uploader.go` | 生成後の AWS S3 アップロード |
| `cmd/fibr-gen/main.go` | CLI エントリポイント |
| `cmd/lambda/main.go` | AWS Lambda エントリポイント |

---

## 生成フロー

```
1. YAML 設定を読み込む
2. GenerationContext を初期化（グローバルパラメータ、データソース、設定レジストリ）
3. Excel テンプレートファイルを開く
4. 各シートに対して：
   ├─ 動的シート？→ パラメータラベルのユニーク値ごとにシートを複製
   └─ 各ブロックに対して：
       ├─ 値ブロック     → データ取得 → 行/列拡張 → セル入力
       ├─ ヘッダーブロック → データ取得 → 重複排除 → 方向に沿って拡張
       └─ マトリックス   → 両軸を識別 → 行/列挿入 → グリッド走査 → 交差データ入力
5. 出力 Excel ファイルを保存
6.（任意）S3 にアップロード
```

---

## パラメータフィルタリング機構

マトリックスデータ入力の仕組みを理解するための鍵です。

### 動作原理

```
グローバルパラメータ → Fetcher.Fetch() で全データ取得 → DataView にキャッシュ
  → 各ブロックが GetBlockDataWithParams(block, params) を呼出
    → DataView.Copy()          キャッシュ汚染防止のためディープコピー
    → DataView.Filter(params)  パラメータでフィルタリング
    → ヘッダーブロック？ → distinctData() でユニーク値を取得
    → 値ブロック？     → フィルタ済みの行をそのまま返す
```

### マトリックスにおけるパラメータ伝播

マトリックス走査中、パラメータは**階層的に積み重ね**られます。例えば売上レポートマトリックスの場合：

```
外側マトリックス：行軸=地域、列軸=製品
内側マトリックス：行軸=指標、列軸=期間
```

「地域=東部、製品=Alpha」の交差セルを処理する際：
1. 外側マトリックスが `{region: "東部", product: "Alpha"}` を `cellParams` に注入
2. 内側マトリックスがこれらのパラメータを受け取る
3. 内側のデータブロックは自動的に `region=東部 AND product=Alpha` でフィルタリング
4. 結果：各交差セルには対応するデータのみが表示される

### 単一データビューの活用

すべてのブロックが**同じ DataView** を参照できます。システムが自動的に処理します：
- **ヘッダーブロック**：`labelVariable` で指定された列で自動重複排除
- **値ブロック**：現在のパラメータに一致するすべての行を返す

つまり、`region, product, metric, period, amount` の5列を持つ単一ビューで、外側の行軸（region で重複排除）、外側の列軸（product で重複排除）、内側のデータブロック（パラメータでフィルタして amount を取得）のすべてに対応できます。

---

## ネストされたマトリックス（再帰的マトリックス）

マトリックスブロックはネストをサポートしています：外側マトリックスのデータ領域内に内側マトリックスを配置できます。

### 制約

- 内側マトリックスは**行や列を拡張できません**（各軸は正確に1つのデータ項目のみ返す必要があります）
- 内側マトリックスのデータは、外側マトリックスから渡されたパラメータで自動的にフィルタリングされます

### レイアウト例

```
      A       B       C       D
 1                   {product}           ← 外側水平軸（C1:D2 結合セル）
 2                   (結合領域)
 3   {region}                 {period}   ← 外側垂直軸（A3:B4 結合）| 内側水平軸
 4   (結合)           {metric} {amount}  ← 内側垂直軸 | 内側値
```

展開後（2地域 × 2製品）：

```
      A       B       C        D        E        F
 1                   Alpha             Beta
 2                   (結合)             (結合)
 3   East                     Q1                Q1
 4   (結合)          Revenue   100     Revenue   200
 5   West                     Q1                Q1
 6   (結合)          Revenue   300     Revenue   400
```

---

## 動的日付

相対日付式をサポートしています。形式：`$date:出力形式:オフセット単位:オフセット量`

| 式 | 意味 | 出力例 |
|----|------|--------|
| `$date:day:day:0` | 今日 | `2025-03-01` |
| `$date:day:day:-1` | 昨日 | `2025-02-28` |
| `$date:month:month:0` | 今月 | `2025-03` |
| `$date:year:year:-1` | 昨年 | `2024` |

ワークブックの `parameters` や、時系列データフィルタリングのための `archiveRule` で使用できます。

---

## テンプレートキャッシュ機構

`generator.go` の `captureTemplate()` / `fillTemplate()` は効率的なテンプレート再利用を実装しています：

1. **captureTemplate()**：テンプレート領域を一度だけ読み取り、すべてのセル値、スタイルID、結合セル情報をキャッシュ
2. **fillTemplate()**：キャッシュされた内容を新しい位置に書き込み、`{label}` プレースホルダーを実データに置換

マトリックス展開時のテンプレート反復読み取りを回避し、大規模レポートの生成パフォーマンスを大幅に向上させます。

---

## 実行方法

### CLI モード

```bash
fibr-gen \
  -config config.yaml \
  -templates ./templates \
  -output ./output \
  -fetcher mysql \
  -db-dsn "user:pass@tcp(localhost)/db"
```

### AWS Lambda モード

`cmd/lambda/main.go` を通じて Lambda 関数としてデプロイします。スケジュール実行やイベント駆動型のレポート生成に適しています。

### オプション：S3 アップロード

```bash
fibr-gen ... -s3-bucket my-bucket -s3-prefix reports/
```

生成完了後、指定された S3 パスにレポートを自動アップロードします。
