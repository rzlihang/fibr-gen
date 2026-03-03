package core

import (
	"fibr-gen/config"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func saveTestFile(t testing.TB, f *excelize.File, name string) {
	dir := "../test_output"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("Failed to create output directory: %v", err)
		return
	}
	path := filepath.Join(dir, name)
	if err := f.SaveAs(path); err != nil {
		t.Logf("Failed to save file %s: %v", path, err)
	} else {
		t.Logf("Saved test file to: %s", path)
	}
}

// MockFetcher is a simple DataFetcher implementation for testing
type MockFetcher struct {
	Data map[string][]map[string]interface{}
}

func (m *MockFetcher) Fetch(viewName string, params map[string]string) ([]map[string]interface{}, error) {
	allData, ok := m.Data[viewName]
	if !ok {
		return nil, fmt.Errorf("view not found: %s", viewName)
	}

	// Filter data based on params if any
	// This mimics the behavior of a real fetcher (e.g. SQL WHERE clause)
	// Only simple string equality is supported for now.
	if len(params) == 0 {
		return allData, nil
	}

	var filtered []map[string]interface{}
	for _, row := range allData {
		match := true
		for k, v := range params {
			// Check if row has this key and if it matches
			// If row doesn't have the key, we assume it's not a filterable column or we ignore it?
			// Usually parameters map to columns.
			if rowVal, exists := row[k]; exists {
				if fmt.Sprintf("%v", rowVal) != v {
					match = false
					break
				}
			}
			// If param is not in row, we ignore it (maybe it's for another block)
		}
		if match {
			filtered = append(filtered, row)
		}
	}

	return filtered, nil
}

// Tests for Excel Operations and Block Processing

func TestMatrixBlock_MergedHeader(t *testing.T) {
	// 1. Setup Excel Template
	f := excelize.NewFile()
	sheet := "Sheet1"
	// Ensure Sheet1 exists and is clean
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}

	// VAxis Template (Merged A2:B2)
	// A2:B2
	f.SetCellValue(sheet, "A2", "{item}")
	if err := f.MergeCell(sheet, "A2", "B2"); err != nil {
		t.Fatalf("Failed to merge cells: %v", err)
	}

	// HAxis Template (C1)
	f.SetCellValue(sheet, "C1", "{header}")

	// Data Template (C2)
	f.SetCellValue(sheet, "C2", "ValueTemplate")

	// 2. Setup Config
	vAxisConf := config.BlockConfig{
		Name:          "VAxis",
		Type:          config.BlockTypeHeader,
		Direction:     config.DirectionVertical,
		Range:         config.CellRange{Ref: "A2:B2"}, // Merged Range
		DataViewName:  "v_items",
		LabelVariable: "item",
		InsertAfter:   true, // Expand Rows
	}

	hAxisConf := config.BlockConfig{
		Name:          "HAxis",
		Type:          config.BlockTypeHeader,
		Direction:     config.DirectionHorizontal,
		Range:         config.CellRange{Ref: "C1:C1"},
		DataViewName:  "v_headers",
		LabelVariable: "header",
	}

	templateBlock := config.BlockConfig{
		Name:  "Template1",
		Type:  config.BlockTypeValue,
		Range: config.CellRange{Ref: "C2:C2"},
	}

	// SubBlocks
	matrixBlock := &config.BlockConfig{
		Name:      "MatrixBlock",
		Type:      config.BlockTypeMatrix,
		Direction: config.DirectionVertical,
		Range:     config.CellRange{Ref: "A1:C2"}, // Bounding Box
		SubBlocks: []config.BlockConfig{vAxisConf, hAxisConf, templateBlock},
	}

	wbConfig := &config.WorkbookConfig{
		Sheets: []config.SheetConfig{
			{Name: "Sheet1", Blocks: []config.BlockConfig{*matrixBlock}},
		},
	}

	// 3. Mock Data
	views := map[string]*config.DataViewConfig{
		"v_items": {
			Name:   "v_items",
			Labels: []config.LabelConfig{{Name: "item", Column: "item_col"}},
		},
		"v_headers": {
			Name:   "v_headers",
			Labels: []config.LabelConfig{{Name: "header", Column: "header_col"}},
		},
	}
	mockData := map[string][]map[string]interface{}{
		"v_items": {
			{"item_col": "Item1"},
			{"item_col": "Item2"},
			{"item_col": "Item3"},
		},
		"v_headers": {
			{"header_col": "H1"},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)
	adapter := &ExcelizeFile{file: f}

	// Process Sheet1
	sheetConf := wbConfig.Sheets[0]
	// Manually process blocks as per Generator.Generate logic (simplified)
	for _, block := range sheetConf.Blocks {
		if err := gen.processBlock(adapter, sheetConf.Name, &block); err != nil {
			t.Fatalf("processBlock failed: %v", err)
		}
	}

	// 4. Verify
	// Expect 3 items (Item1, Item2, Item3)
	// Item1 at A2:B2 (Merged)
	// Item2 at A3:B3 (Merged)
	// Item3 at A4:B4 (Merged)

	// Check Merge Cells
	mergeCells, err := f.GetMergeCells(sheet)
	if err != nil {
		t.Fatalf("Failed to get merge cells: %v", err)
	}

	// We expect 3 merged cells corresponding to the items
	if len(mergeCells) < 3 {
		t.Logf("Found %d merged cells", len(mergeCells))
	}

	expectedMerges := []string{"A2:B2", "A3:B3", "A4:B4"}
	for _, expected := range expectedMerges {
		found := false
		for _, mc := range mergeCells {
			ref := mc.GetStartAxis() + ":" + mc.GetEndAxis()
			if ref == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected merged cell %s not found", expected)
		} else {
			t.Logf("Merged Cell: %s", expected)
		}
	}

	// Check Values
	val, _ := f.GetCellValue(sheet, "A2")
	if val != "Item1" {
		t.Errorf("A2: want Item1, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "A3")
	if val != "Item2" {
		t.Errorf("A3: want Item2, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "A4")
	if val != "Item3" {
		t.Errorf("A4: want Item3, got %s", val)
	}
}

// Helper to create Demo Report Template
func setupTemplateDemo(t *testing.T) *excelize.File {
	f := excelize.NewFile()
	sheet := "Sheet1"
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}
	// Title Block A1:B1
	f.SetCellValue(sheet, "A1", "Title")
	f.SetCellValue(sheet, "B1", "{report_date}")
	return f
}

func TestEndToEnd_DemoReport(t *testing.T) {
	// Config: test/workbooks/demo_report.yaml
	// This is a simple ValueBlock test (TitleBlock)

	wbConfig := &config.WorkbookConfig{
		Id:        "wb_demo",
		Name:      "DemoReport",
		Template:  "demo_template.xlsx",
		OutputDir: "reports",
		Sheets: []config.SheetConfig{
			{
				Name:    "Sheet1",
				Dynamic: false,
				Blocks: []config.BlockConfig{
					{
						Name:         "TitleBlock",
						Type:         config.BlockTypeValue,
						Range:        config.CellRange{Ref: "A1:B1"},
						DataViewName: "v_title",
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"v_title": {
			Name: "v_title",
			Labels: []config.LabelConfig{
				{Name: "report_date", Column: "RPT_DATE"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"v_title": {
			{"RPT_DATE": "2023-10-27"},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)

	f := setupTemplateDemo(t)
	adapter := &ExcelizeFile{file: f}

	block := &wbConfig.Sheets[0].Blocks[0]
	if err := gen.processBlock(adapter, "Sheet1", block); err != nil {
		t.Fatalf("processBlock failed: %v", err)
	}

	saveTestFile(t, f, "demo_report.xlsx")

	// Verify
	val, _ := f.GetCellValue("Sheet1", "B1")
	if val != "2006-01-02" { // MockFetcher doesn't format dates, it passes string "2023-10-27"
		// Wait, my mock data is "2023-10-27".
	}
	if val != "2023-10-27" {
		t.Errorf("Expected 2023-10-27, got %s", val)
	}
}

// Helper to create ValueBlock Template
func setupTemplateValueBlock(t testing.TB) *excelize.File {
	f := excelize.NewFile()
	sheet := "Sheet1"
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}
	// EmployeeList A2:C2
	f.SetCellValue(sheet, "A2", "{dept}")
	f.SetCellValue(sheet, "B2", "{name}")
	f.SetCellValue(sheet, "C2", "{salary}")
	return f
}

func TestEndToEnd_ValueBlock(t *testing.T) {
	// Config: test/workbooks/valueblock_test.yaml

	wbConfig := &config.WorkbookConfig{
		Id:        "wb_valueblock_test",
		Name:      "ValueBlockTest",
		Template:  "valueblock_template.xlsx",
		OutputDir: "tests",
		Sheets: []config.SheetConfig{
			{
				Name:    "Sheet1",
				Dynamic: false,
				Blocks: []config.BlockConfig{
					{
						Name:         "EmployeeList",
						Type:         config.BlockTypeValue,
						Range:        config.CellRange{Ref: "A2:C2"},
						DataViewName: "employee_view",
						Direction:    config.DirectionVertical,
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"employee_view": {
			Name: "employee_view",
			Labels: []config.LabelConfig{
				{Name: "dept", Column: "DEPT_CD"},
				{Name: "name", Column: "USER_NAME"},
				{Name: "salary", Column: "SALARY"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"employee_view": {
			{"DEPT_CD": "D001", "USER_NAME": "Alice", "SALARY": 5000},
			{"DEPT_CD": "D001", "USER_NAME": "Bob", "SALARY": 6000},
			{"DEPT_CD": "D002", "USER_NAME": "Charlie", "SALARY": 7000},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)

	f := setupTemplateValueBlock(t)
	adapter := &ExcelizeFile{file: f}

	block := &wbConfig.Sheets[0].Blocks[0]
	if err := gen.processBlock(adapter, "Sheet1", block); err != nil {
		t.Fatalf("processBlock failed: %v", err)
	}

	saveTestFile(t, f, "value_block.xlsx")

	// Verify Expansion
	// Row 2: Alice
	// Row 3: Bob
	// Row 4: Charlie

	val, _ := f.GetCellValue("Sheet1", "B2")
	if val != "Alice" {
		t.Errorf("Row 2 Name: want Alice, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "B3")
	if val != "Bob" {
		t.Errorf("Row 3 Name: want Bob, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "B4")
	if val != "Charlie" {
		t.Errorf("Row 4 Name: want Charlie, got %s", val)
	}
}

// Helper to create Cross Template
func setupTemplateCross(t *testing.T) *excelize.File {
	f := excelize.NewFile()
	sheet := "Sheet1"
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}
	// MatrixBlock A2:B3
	// MonthAxis B2 (Horizontal)
	f.SetCellValue(sheet, "B2", "{month_label}")

	// EmpAxis A3 (Vertical)
	f.SetCellValue(sheet, "A3", "{emp_name}")

	// ScoreData B3 (Intersection)
	f.SetCellValue(sheet, "B3", "{score}")

	return f
}

func TestEndToEnd_CrossTest(t *testing.T) {
	// Config: test/workbooks/cross_test.yaml

	// SubBlocks
	vAxisConf := config.BlockConfig{
		Name:         "EmpAxis",
		Type:         config.BlockTypeHeader,
		Direction:    config.DirectionVertical,
		Range:        config.CellRange{Ref: "A3:A3"},
		DataViewName: "v_emp",
		InsertAfter:  true,
	}

	hAxisConf := config.BlockConfig{
		Name:         "MonthAxis",
		Type:         config.BlockTypeHeader,
		Direction:    config.DirectionHorizontal,
		Range:        config.CellRange{Ref: "B2:B2"},
		DataViewName: "v_month",
	}

	dataBlock := config.BlockConfig{
		Name:         "ScoreData",
		Type:         config.BlockTypeValue,
		Range:        config.CellRange{Ref: "B3:B3"},
		DataViewName: "v_full_perf",
	}

	matrixBlock := config.BlockConfig{
		Name:      "PerformanceMatrix",
		Type:      config.BlockTypeMatrix,
		Range:     config.CellRange{Ref: "A2:B3"}, // Covers headers and data
		SubBlocks: []config.BlockConfig{vAxisConf, hAxisConf, dataBlock},
	}

	wbConfig := &config.WorkbookConfig{
		Id:        "wb_cross_test",
		Name:      "CrossTest",
		Template:  "cross_template.xlsx",
		OutputDir: "tests",
		Sheets: []config.SheetConfig{
			{
				Name:    "Sheet1",
				Dynamic: false,
				Blocks:  []config.BlockConfig{matrixBlock},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"v_emp": {
			Name:   "v_emp",
			Labels: []config.LabelConfig{{Name: "emp_name", Column: "EMP_NAME"}},
		},
		"v_month": {
			Name:   "v_month",
			Labels: []config.LabelConfig{{Name: "month_label", Column: "MONTH_LABEL"}},
		},
		"v_full_perf": {
			Name: "v_full_perf",
			Labels: []config.LabelConfig{
				{Name: "emp_id", Column: "EMP_ID"},
				{Name: "emp_name", Column: "EMP_NAME"},
				{Name: "month_id", Column: "MONTH_ID"},
				{Name: "month_label", Column: "MONTH_LABEL"},
				{Name: "score", Column: "SCORE"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"v_emp": {
			{"EMP_NAME": "Alice"},
			{"EMP_NAME": "Bob"},
		},
		"v_month": {
			{"MONTH_LABEL": "Jan"},
			{"MONTH_LABEL": "Feb"},
		},
		"v_full_perf": {
			{"EMP_ID": "E001", "EMP_NAME": "Alice", "MONTH_ID": "M01", "MONTH_LABEL": "Jan", "SCORE": 85},
			{"EMP_ID": "E001", "EMP_NAME": "Alice", "MONTH_ID": "M02", "MONTH_LABEL": "Feb", "SCORE": 88},
			{"EMP_ID": "E002", "EMP_NAME": "Bob", "MONTH_ID": "M01", "MONTH_LABEL": "Jan", "SCORE": 75},
			{"EMP_ID": "E002", "EMP_NAME": "Bob", "MONTH_ID": "M02", "MONTH_LABEL": "Feb", "SCORE": 78},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)

	f := setupTemplateCross(t)
	adapter := &ExcelizeFile{file: f}

	if err := gen.processBlock(adapter, "Sheet1", &matrixBlock); err != nil {
		t.Fatalf("processBlock failed: %v", err)
	}

	saveTestFile(t, f, "cross_test.xlsx")

	// Verify
	// Axis H (Month): Jan (B2), Feb (C2)
	// Axis V (Emp): Alice (A3), Bob (A4)
	// Data:
	// B3 (Alice, Jan): 85
	// C3 (Alice, Feb): 88
	// B4 (Bob, Jan): 75
	// C4 (Bob, Feb): 78

	val, _ := f.GetCellValue("Sheet1", "B2")
	if val != "Jan" {
		t.Errorf("B2: want Jan, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "C2")
	if val != "Feb" {
		t.Errorf("C2: want Feb, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "A3")
	if val != "Alice" {
		t.Errorf("A3: want Alice, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "A4")
	if val != "Bob" {
		t.Errorf("A4: want Bob, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "B3")
	if val != "85" {
		t.Errorf("B3: want 85, got %s", val)
	}
	val, _ = f.GetCellValue("Sheet1", "C4")
	if val != "78" {
		t.Errorf("C4: want 78, got %s", val)
	}
}

func TestMatrixBlock_NestedMatrix(t *testing.T) {
	// Recursive matrix: outer matrix with merged-cell headers, inner ordinary matrix.
	// Single data view "sales_data" with 5 columns: region, product, metric, period, amount.
	//
	// Template layout:
	//      A       B       C       D
	// 1                   {product}          ← C1:D1 merged, outer HAxis (horizontal)
	// 2                   {period}           ← inner HAxis at D2 (corner C2 empty)
	// 3   {region}        {metric} {amount}  ← A3:B3 merged, outer VAxis (vertical, InsertAfter)
	//     (merged A3:B3)   inner VAxis C3, inner Values D3
	//
	// After expansion (2 regions × 2 products):
	//      A       B       C       D       E       F
	// 1                   Alpha            Beta
	// 2                            Q1               Q1
	// 3   East            Revenue  100     Revenue  200
	// 4   West            Revenue  300     Revenue  400

	f := excelize.NewFile()
	sheet := "Sheet1"
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}

	// Outer HAxis template (merged C1:D1)
	f.SetCellValue(sheet, "C1", "{product}")
	if err := f.MergeCell(sheet, "C1", "D1"); err != nil {
		t.Fatalf("Failed to merge C1:D1: %v", err)
	}

	// Outer VAxis template (merged A2:B2)
	f.SetCellValue(sheet, "A2", "{region}")
	if err := f.MergeCell(sheet, "A2", "B2"); err != nil {
		t.Fatalf("Failed to merge A2:B2: %v", err)
	}

	// Inner matrix template cells (C2:D2)
	f.SetCellValue(sheet, "D1", "{period}") // inner HAxis (Note: D1 is within outer HAxis merge, but inner HAxis template is at D1 position)
	// Actually inner HAxis is at D1 within inner matrix range... let me reconsider.

	// Wait, let me reconsider the layout. The inner matrix must be in the "data area"
	// which is the intersection of outer VAxis rows and outer HAxis cols.
	// Outer VAxis: A2:B2 (row 2), Outer HAxis: C1:D1 (row 1, cols C-D)
	// So data area = C2:D2 (row 2, cols C-D)
	// Inner matrix at C2:D2 is only 1 row × 2 cols — too small for 2 headers + 1 value.

	// Let me use a 2-row layout instead:
	// Outer VAxis: A3:B4 merged (2 rows × 2 cols)
	// Outer HAxis: C1:D2 merged (2 rows × 2 cols)
	// Inner matrix: C3:D4 (2 rows × 2 cols)
	//   Corner: C3
	//   Inner HAxis: D3 ({period})
	//   Inner VAxis: C4 ({metric})
	//   Inner Values: D4 ({amount})

	// Reset and redo
	f = excelize.NewFile()
	idx, _ = f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}

	// Outer HAxis template: C1:D2 merged
	f.SetCellValue(sheet, "C1", "{product}")
	if err := f.MergeCell(sheet, "C1", "D2"); err != nil {
		t.Fatalf("Failed to merge C1:D2: %v", err)
	}

	// Outer VAxis template: A3:B4 merged
	f.SetCellValue(sheet, "A3", "{region}")
	if err := f.MergeCell(sheet, "A3", "B4"); err != nil {
		t.Fatalf("Failed to merge A3:B4: %v", err)
	}

	// Inner matrix template cells at C3:D4
	// C3 = corner (empty)
	f.SetCellValue(sheet, "D3", "{period}")  // inner HAxis
	f.SetCellValue(sheet, "C4", "{metric}")  // inner VAxis
	f.SetCellValue(sheet, "D4", "{amount}")  // inner Values

	// All blocks reference the same data view
	dvName := "sales_data"

	outerVAxis := config.BlockConfig{
		Name:          "OuterRows",
		Type:          config.BlockTypeHeader,
		Direction:     config.DirectionVertical,
		Range:         config.CellRange{Ref: "A3:B4"},
		DataViewName:  dvName,
		LabelVariable: "region",
		InsertAfter:   true,
	}

	outerHAxis := config.BlockConfig{
		Name:          "OuterCols",
		Type:          config.BlockTypeHeader,
		Direction:     config.DirectionHorizontal,
		Range:         config.CellRange{Ref: "C1:D2"},
		DataViewName:  dvName,
		LabelVariable: "product",
	}

	innerMatrix := config.BlockConfig{
		Name:  "InnerMatrix",
		Type:  config.BlockTypeMatrix,
		Range: config.CellRange{Ref: "C3:D4"},
		SubBlocks: []config.BlockConfig{
			{
				Name:          "InnerRows",
				Type:          config.BlockTypeHeader,
				Direction:     config.DirectionVertical,
				Range:         config.CellRange{Ref: "C4:C4"},
				DataViewName:  dvName,
				LabelVariable: "metric",
			},
			{
				Name:          "InnerCols",
				Type:          config.BlockTypeHeader,
				Direction:     config.DirectionHorizontal,
				Range:         config.CellRange{Ref: "D3:D3"},
				DataViewName:  dvName,
				LabelVariable: "period",
			},
			{
				Name:         "InnerValues",
				Type:         config.BlockTypeValue,
				Range:        config.CellRange{Ref: "D4:D4"},
				DataViewName: dvName,
				RowLimit:     1,
			},
		},
	}

	matrixBlock := config.BlockConfig{
		Name:      "OuterMatrix",
		Type:      config.BlockTypeMatrix,
		Range:     config.CellRange{Ref: "A1:D4"},
		SubBlocks: []config.BlockConfig{outerVAxis, outerHAxis, innerMatrix},
	}

	wbConfig := &config.WorkbookConfig{
		Sheets: []config.SheetConfig{
			{Name: "Sheet1", Blocks: []config.BlockConfig{matrixBlock}},
		},
	}

	// Single data view with 5 columns
	views := map[string]*config.DataViewConfig{
		dvName: {
			Name: dvName,
			Labels: []config.LabelConfig{
				{Name: "region", Column: "region"},
				{Name: "product", Column: "product"},
				{Name: "metric", Column: "metric"},
				{Name: "period", Column: "period"},
				{Name: "amount", Column: "amount"},
			},
		},
	}

	// All data in one view — headers use distinct, values use filtered rows
	mockData := map[string][]map[string]interface{}{
		dvName: {
			{"region": "East", "product": "Alpha", "metric": "Revenue", "period": "Q1", "amount": 100},
			{"region": "East", "product": "Beta", "metric": "Revenue", "period": "Q1", "amount": 200},
			{"region": "West", "product": "Alpha", "metric": "Revenue", "period": "Q1", "amount": 300},
			{"region": "West", "product": "Beta", "metric": "Revenue", "period": "Q1", "amount": 400},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)
	adapter := &ExcelizeFile{file: f}

	if err := gen.processBlock(adapter, sheet, &matrixBlock); err != nil {
		t.Fatalf("processBlock failed: %v", err)
	}

	saveTestFile(t, f, "nested_matrix.xlsx")

	// Expected layout after expansion (2 regions × 2 products):
	//      A       B       C        D        E        F
	// 1                   Alpha             Beta
	// 2                   (C1:D2 merged)    (E1:F2 merged)
	// 3   East                     Q1                Q1
	// 4   (A3:B4)         Revenue  100      Revenue  200
	// 5   West                     Q1                Q1
	// 6   (A5:B6)         Revenue  300      Revenue  400

	// Verify outer VAxis merged headers
	val, _ := f.GetCellValue(sheet, "A3")
	if val != "East" {
		t.Errorf("A3 (outer VAxis): want East, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "A5")
	if val != "West" {
		t.Errorf("A5 (outer VAxis): want West, got %s", val)
	}

	// Verify outer HAxis merged headers
	val, _ = f.GetCellValue(sheet, "C1")
	if val != "Alpha" {
		t.Errorf("C1 (outer HAxis): want Alpha, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "E1")
	if val != "Beta" {
		t.Errorf("E1 (outer HAxis): want Beta, got %s", val)
	}

	// Verify merged cells exist for outer headers
	mergeCells, err := f.GetMergeCells(sheet)
	if err != nil {
		t.Fatalf("Failed to get merge cells: %v", err)
	}
	expectedMerges := []string{"A3:B4", "A5:B6", "C1:D2", "E1:F2"}
	for _, expected := range expectedMerges {
		found := false
		for _, mc := range mergeCells {
			ref := mc.GetStartAxis() + ":" + mc.GetEndAxis()
			if ref == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected merged cell %s not found", expected)
		}
	}

	// Verify inner HAxis headers (period)
	val, _ = f.GetCellValue(sheet, "D3")
	if val != "Q1" {
		t.Errorf("D3 (inner period, East×Alpha): want Q1, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "F3")
	if val != "Q1" {
		t.Errorf("F3 (inner period, East×Beta): want Q1, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "D5")
	if val != "Q1" {
		t.Errorf("D5 (inner period, West×Alpha): want Q1, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "F5")
	if val != "Q1" {
		t.Errorf("F5 (inner period, West×Beta): want Q1, got %s", val)
	}

	// Verify inner VAxis headers (metric)
	val, _ = f.GetCellValue(sheet, "C4")
	if val != "Revenue" {
		t.Errorf("C4 (inner metric, East×Alpha): want Revenue, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "E4")
	if val != "Revenue" {
		t.Errorf("E4 (inner metric, East×Beta): want Revenue, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "C6")
	if val != "Revenue" {
		t.Errorf("C6 (inner metric, West×Alpha): want Revenue, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "E6")
	if val != "Revenue" {
		t.Errorf("E6 (inner metric, West×Beta): want Revenue, got %s", val)
	}

	// Verify inner values (amount)
	val, _ = f.GetCellValue(sheet, "D4")
	if val != "100" {
		t.Errorf("D4 (East×Alpha value): want 100, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "F4")
	if val != "200" {
		t.Errorf("F4 (East×Beta value): want 200, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "D6")
	if val != "300" {
		t.Errorf("D6 (West×Alpha value): want 300, got %s", val)
	}
	val, _ = f.GetCellValue(sheet, "F6")
	if val != "400" {
		t.Errorf("F6 (West×Beta value): want 400, got %s", val)
	}
}

// Helper to create ArchiveDate Template
func setupTemplateArchiveDate(t *testing.T) *excelize.File {
	f := excelize.NewFile()
	sheet := "Sheet1"
	idx, _ := f.GetSheetIndex("Sheet1")
	if idx == -1 {
		f.NewSheet(sheet)
	}
	// Row 2: {date} {value}
	f.SetCellValue(sheet, "A2", "{date}")
	f.SetCellValue(sheet, "B2", "{value}")
	return f
}

func TestEndToEnd_ArchiveDate(t *testing.T) {
	// Scenario: ArchiveRule enabled for "date" column.

	// Mock Today
	today := "2023-10-27"

	wbConfig := &config.WorkbookConfig{
		Id: "wb_archive",
		// ArchiveRule is string in types.go, so we pass a string.
		// Logic isn't implemented fully in types/generator yet, so we assume
		// the string represents configuration "enabled: true, dateColumn: RPT_DATE".
		// But here we just test filtering logic simulation.
		ArchiveRule: "enabled",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:         "Data",
						Type:         config.BlockTypeValue,
						Range:        config.CellRange{Ref: "A2:B2"},
						DataViewName: "v_data",
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"v_data": {
			Name: "v_data",
			Labels: []config.LabelConfig{
				{Name: "date", Column: "RPT_DATE"},
				{Name: "value", Column: "VAL"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"v_data": {
			{"RPT_DATE": today, "VAL": 100},
			{"RPT_DATE": "2023-10-26", "VAL": 90}, // Yesterday
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)

	// Manually inject filter for test purpose
	// In real app, Context or Fetcher should interpret ArchiveRule.
	if ctx.WorkbookConfig.ArchiveRule != "" {
		ctx.Parameters["RPT_DATE"] = today
	}

	gen := NewGenerator(ctx)
	f := setupTemplateArchiveDate(t)
	adapter := &ExcelizeFile{file: f}

	block := &wbConfig.Sheets[0].Blocks[0]
	if err := gen.processBlock(adapter, "Sheet1", block); err != nil {
		t.Fatalf("processBlock failed: %v", err)
	}

	saveTestFile(t, f, "archive_date.xlsx")

	// Verify
	// Row 2: Today
	val, _ := f.GetCellValue("Sheet1", "A2")
	if val != today {
		t.Errorf("A2: want %s, got %s", today, val)
	}

	// Row 3: Should be empty if filtered
	val, _ = f.GetCellValue("Sheet1", "A3")
	if val != "" {
		t.Errorf("A3: want empty, got %s (Filtering failed?)", val)
	}
}

func TestDynamicSheet_MatrixBlock_ParamInheritance(t *testing.T) {
	// Scenario:
	// Dynamic Sheet iterates over "months" (M1, M2).
	// Matrix Block (Cross Table) inside needs to filter "sales" by "month_id".
	// If bug exists, the matrix block will miss "month_id" and might show wrong data.

	wbConfig := &config.WorkbookConfig{
		Id:       "wb_dyn_param",
		Name:     "DynamicParamTest",
		Template: "dyn_template.xlsx",
		Sheets: []config.SheetConfig{
			{
				Name:         "Sheet",
				Dynamic:      true,
				DataViewName: "v_months",
				ParamLabel:   "month_id",
				Blocks: []config.BlockConfig{
					{
						Name:  "SalesMatrix",
						Type:  config.BlockTypeMatrix,
						Range: config.CellRange{Ref: "A1:B2"},
						SubBlocks: []config.BlockConfig{
							{
								Name:          "ItemAxis",
								Type:          config.BlockTypeHeader,
								Direction:     config.DirectionVertical,
								InsertAfter:   true,
								Range:         config.CellRange{Ref: "A2:A2"},
								DataViewName:  "v_items",
								LabelVariable: "item_id",
							},
							{
								Name:         "MetricAxis",
								Type:         config.BlockTypeHeader,
								Direction:    config.DirectionHorizontal,
								Range:        config.CellRange{Ref: "B1:B1"},
								DataViewName: "v_metrics", // Static "Revenue"
							},
							{
								Name:         "Data",
								Type:         config.BlockTypeValue,
								Range:        config.CellRange{Ref: "B2:B2"},
								DataViewName: "v_sales",
								RowLimit:     1,
							},
						},
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"v_months": {
			Name:   "v_months",
			Labels: []config.LabelConfig{{Name: "month_id", Column: "month_id"}},
		},
		"v_items": {
			Name:   "v_items",
			Labels: []config.LabelConfig{{Name: "item_id", Column: "item_id"}},
		},
		"v_metrics": {
			Name:   "v_metrics",
			Labels: []config.LabelConfig{{Name: "metric", Column: "metric"}},
		},
		"v_sales": {
			Name: "v_sales",
			Labels: []config.LabelConfig{
				{Name: "month_id", Column: "month_id"},
				{Name: "item_id", Column: "item_id"},
				{Name: "revenue", Column: "revenue"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"v_months": {
			{"month_id": "M1"},
			{"month_id": "M2"},
		},
		"v_items": {
			{"item_id": "I1"}, // Just one item for simplicity
		},
		"v_metrics": {
			{"metric": "Revenue"},
		},
		"v_sales": {
			{"month_id": "M1", "item_id": "I1", "revenue": 100},
			{"month_id": "M2", "item_id": "I1", "revenue": 200},
		},
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)
	ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
	gen := NewGenerator(ctx)

	// Setup Template
	f := excelize.NewFile()
	// We need a template sheet to copy from. Dynamic logic copies "Sheet" to "M1", "M2".
	// The template "Sheet" must exist.
	f.SetSheetName("Sheet1", "Sheet")
	f.SetCellValue("Sheet", "A2", "{item_id}")
	f.SetCellValue("Sheet", "B1", "Revenue")
	f.SetCellValue("Sheet", "B2", "{revenue}")

	adapter := &ExcelizeFile{file: f}

	// Run processDynamicSheet
	sheetConf := &wbConfig.Sheets[0]
	err := gen.processSheet(adapter, sheetConf)
	if err != nil {
		t.Fatalf("processSheet failed: %v", err)
	}

	saveTestFile(t, f, "dynamic_sheet.xlsx")

	// Verify M1
	// M1 should have 100
	val, _ := f.GetCellValue("M1", "B2")
	if val != "100" {
		t.Errorf("M1 B2: want 100, got %s", val)
	}

	// Verify M2
	// M2 should have 200
	val, _ = f.GetCellValue("M2", "B2")
	if val != "200" {
		t.Errorf("M2 B2: want 200, got %s", val)
	}
}

func BenchmarkValueBlock_Insert50k(b *testing.B) {
	// 1. Generate 50k rows
	count := 50000
	rows := make([]map[string]interface{}, count)
	for i := range count {
		rows[i] = map[string]interface{}{
			"DEPT_CD":   "D001",
			"USER_NAME": fmt.Sprintf("User%d", i),
			"SALARY":    1000 + i,
		}
	}

	// 2. Setup Config
	wbConfig := &config.WorkbookConfig{
		Id: "wb_perf_test",
		Sheets: []config.SheetConfig{
			{
				Name: "Sheet1",
				Blocks: []config.BlockConfig{
					{
						Name:         "EmployeeList",
						Type:         config.BlockTypeValue,
						Range:        config.CellRange{Ref: "A2:C2"},
						DataViewName: "employee_view",
						Direction:    config.DirectionVertical,
					},
				},
			},
		},
	}

	views := map[string]*config.DataViewConfig{
		"employee_view": {
			Name: "employee_view",
			Labels: []config.LabelConfig{
				{Name: "dept", Column: "DEPT_CD"},
				{Name: "name", Column: "USER_NAME"},
				{Name: "salary", Column: "SALARY"},
			},
		},
	}

	mockData := map[string][]map[string]interface{}{
		"employee_view": rows,
	}

	fetcher := &MockFetcher{Data: mockData}
	provider := config.NewMemoryConfigRegistry(views, nil)

	// Benchmark Loop
	saved := false
	for b.Loop() {
		b.StopTimer() // Pause for setup
		ctx := NewGenerationContext(wbConfig, provider, fetcher, nil)
		gen := NewGenerator(ctx)
		f := setupTemplateValueBlock(b)
		adapter := &ExcelizeFile{file: f}
		block := &wbConfig.Sheets[0].Blocks[0]
		b.StartTimer() // Start measuring

		if err := gen.processBlock(adapter, "Sheet1", block); err != nil {
			b.Fatalf("processBlock failed: %v", err)
		}

		if !saved {
			b.StopTimer()
			saveTestFile(b, f, "benchmark_50k.xlsx")
			saved = true
			b.StartTimer()
		}
	}
}
