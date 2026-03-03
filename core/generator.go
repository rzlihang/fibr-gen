package core

import (
	"fibr-gen/config"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Generator struct {
	Context *GenerationContext
}

func NewGenerator(ctx *GenerationContext) *Generator {
	return &Generator{Context: ctx}
}

func replacePlaceholders(input string, params map[string]string) string {
	output := input
	for k, v := range params {
		placeholder := fmt.Sprintf("${%s}", k)
		output = strings.ReplaceAll(output, placeholder, v)
	}
	return output
}

func cloneParams(params map[string]string) map[string]string {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	return copied
}

// Generate executes the workbook generation process.
func (g *Generator) Generate(templateRoot, outputRoot string) (err error) {
	wbConf := g.Context.WorkbookConfig
	templatePath := filepath.Join(templateRoot, wbConf.Template)

	// Replace parameters in output path
	outputPathStr := replacePlaceholders(wbConf.OutputDir, g.Context.Parameters)

	outputPath := filepath.Join(outputRoot, outputPathStr)
	if filepath.Ext(outputPath) == "" {
		// Replace parameters in workbook name
		name := replacePlaceholders(wbConf.Name, g.Context.Parameters)
		outputPath = filepath.Join(outputPath, name+".xlsx")
	}

	f, err := openExcelFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer func(f ExcelFile) {
		if closeErr := f.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to close template file: %w", closeErr)
			} else {
				err = fmt.Errorf("%w; (cleanup error: %v)", err, closeErr)
			}
		}
	}(f)

	for _, sheetConf := range wbConf.Sheets {
		if err := g.processSheet(f, &sheetConf); err != nil {
			return fmt.Errorf("processing sheet %s: %w", sheetConf.Name, err)
		}
	}

	// UX: Reset view to A1 for all sheets and set first sheet active
	if sheets := f.GetSheetList(); len(sheets) > 0 {
		for _, sheet := range sheets {
			// Ignore error for SetSelection as it's UX improvement
			_ = f.SetSelection(sheet, "A1")
		}
		f.SetActiveSheet(0)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("failed to save output: %w", err)
	}
	return nil
}

func (g *Generator) processSheet(f ExcelFile, sheetConf *config.SheetConfig) error {
	if sheetConf.Dynamic {
		return g.processDynamicSheet(f, sheetConf)
	}

	for _, block := range sheetConf.Blocks {
		if err := g.processBlock(f, sheetConf.Name, &block); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) processBlock(f ExcelFile, sheetName string, block *config.BlockConfig) error {
	return g.processBlockWithParams(f, sheetName, block, g.Context.Parameters)
}

func (g *Generator) processDynamicSheet(f ExcelFile, sheetConf *config.SheetConfig) error {
	// 1. Get Distinct Label Values (e.g. Month Names)
	// Need to find DataView Config
	conf, err := g.Context.ConfigProvider.GetDataViewConfig(sheetConf.DataViewName)
	if err != nil {
		return fmt.Errorf("data view not found for dynamic sheet: %s", sheetConf.DataViewName)
	}

	// Fetch data to get distinct values for ParamLabel
	// We need all rows first
	data, err := g.Context.Fetcher.Fetch(conf.Name, g.Context.Parameters)
	if err != nil {
		return fmt.Errorf("failed to fetch dynamic sheet data: %w", err)
	}

	// Distinct values
	distinctValues := make(map[string]struct{})
	var values []string

	// Find which column maps to ParamLabel
	var paramColumn string
	for _, label := range conf.Labels {
		if label.Name == sheetConf.ParamLabel {
			paramColumn = label.Column
			break
		}
	}
	if paramColumn == "" {
		return fmt.Errorf("param label '%s' not found in data view %s", sheetConf.ParamLabel, sheetConf.DataViewName)
	}

	for _, row := range data {
		if val, ok := row[paramColumn]; ok {
			strVal := fmt.Sprintf("%v", val)
			if _, exists := distinctValues[strVal]; !exists {
				distinctValues[strVal] = struct{}{}
				values = append(values, strVal)
			}
		}
	}

	// 2. Clone Sheets
	templateSheetName := sheetConf.Name
	templateIdx, err := f.GetSheetIndex(templateSheetName)
	if err != nil {
		return fmt.Errorf("template sheet not found: %s", templateSheetName)
	}

	for _, val := range values {
		// Validate sheet name (simple version)
		newSheetName := val
		// Handle conflict if exists? Excelize NewSheet might not handle it?
		// Assuming unique values for now.

		newIdx, err := f.NewSheet(newSheetName)
		if err != nil {
			return fmt.Errorf("failed to create sheet %s: %w", newSheetName, err)
		}

		// Copy content
		if err := f.CopySheet(templateIdx, newIdx); err != nil {
			return fmt.Errorf("failed to copy sheet: %w", err)
		}

		// 3. Process Blocks for this new sheet
		// We need to inject the parameter for this sheet (e.g. month=January)
		sheetParams := cloneParams(g.Context.Parameters)
		sheetParams[sheetConf.ParamLabel] = val

		// Process each block in the NEW sheet
		for _, block := range sheetConf.Blocks {
			// We need to pass the sheetParams down.
			// But processBlock calls processValueBlock which uses g.Context.Parameters.
			// We should refactor processBlock to accept params too, or temporarily modify context?
			// Modifying context is not thread-safe if we parallelize.
			// Passing params is better.

			// We need a helper processBlockWithParams
			if err := g.processBlockWithParams(f, newSheetName, &block, sheetParams); err != nil {
				return err
			}
		}
	}

	// Delete Template Sheet if we generated others?
	if len(values) > 0 {
		f.DeleteSheet(templateSheetName)
	}

	return nil
}

func (g *Generator) processBlockWithParams(f ExcelFile, sheetName string, block *config.BlockConfig, params map[string]string) error {
	switch block.Type {
	case config.BlockTypeValue:
		return g.processValueBlockWithParams(f, sheetName, block, params)
	case config.BlockTypeMatrix:
		// MatrixBlock also needs to accept params!
		// But processMatrixBlock signature is fixed currently.
		// Let's assume we can change it or Context has the params?
		// Ideally we update Context for this operation?
		// Or refactor processMatrixBlock to take params.
		// For now, let's update Context (since Generator is per-request, single threaded usually)
		// But wait, we are inside a loop.

		// Refactoring processMatrixBlock to accept params is the right way.
		return g.processMatrixBlockWithParams(f, sheetName, block, params, true)
	default:
		return fmt.Errorf("unsupported block type: %s", block.Type)
	}
}

func (g *Generator) processMatrixBlockWithParams(f ExcelFile, sheetName string, block *config.BlockConfig, params map[string]string, allowExpansion bool) error {
	// 1. Identify Axes
	var vH, hH *config.BlockConfig
	for i := range block.SubBlocks {
		sb := &block.SubBlocks[i]
		if sb.Type == config.BlockTypeHeader {
			switch sb.Direction {
			case config.DirectionVertical, "":
				vH = sb
			case config.DirectionHorizontal:
				hH = sb
			}
		}
	}

	if vH == nil || hH == nil {
		return fmt.Errorf("MatrixBlock %s must have both vertical and horizontal axes", block.Name)
	}

	// 2. Determine Expansion Mode
	isVerticalExpand := vH.InsertAfter

	var headerData []map[string]interface{}
	var staticData []map[string]interface{}
	var err error

	// 3. Process Axes
	if isVerticalExpand {
		// Vertical Expand Mode
		headerData, err = g.Context.GetBlockDataWithParams(vH, params)
		if err != nil {
			return err
		}

		staticData, err = g.Context.GetBlockDataWithParams(hH, params)
		if err != nil {
			return err
		}

		if !allowExpansion && (len(headerData) > 1 || len(staticData) > 1) {
			return fmt.Errorf("nested matrix block %s cannot expand rows or columns", block.Name)
		}

		// Insert Rows logic (same as before)
		if allowExpansion {
			dataCount := len(headerData)
			if dataCount > 1 {
				_, _, _, endRow, err := parseRange(vH.Range.Ref)
				if err != nil {
					return err
				}
				_, startRow, _, _, err := parseRange(vH.Range.Ref)
				if err != nil {
					return err
				}
				axisHeight := endRow - startRow + 1
				insertCount := (dataCount - 1) * axisHeight
				if err := f.InsertRows(sheetName, endRow+1, insertCount); err != nil {
					return err
				}
			}

			// If Horizontal Axis has multiple items, we need to expand columns too, even if Vertical Header expanded rows
			if len(staticData) > 1 {
				startCol, _, endCol, _, err := parseRange(hH.Range.Ref)
				if err != nil {
					return err
				}
				axisWidth := endCol - startCol + 1
				insertCount := (len(staticData) - 1) * axisWidth
				colName, _ := excelize.ColumnNumberToName(endCol + 1)
				if err := f.InsertCols(sheetName, colName, insertCount); err != nil {
					return err
				}

				// Copy Template Columns
				if err := g.copyTemplateSlice(f, sheetName, block, vH.Name, endCol+1, insertCount, false); err != nil {
					return err
				}
			}

			// Copy Template Rows to the new inserted rows (AFTER cols are inserted to ensure correct width?)
			// Actually, InsertCols might shift rows? No.
			// But InsertRows must happen before we fill data.
			// And we already inserted rows above.
			// BUT we haven't copied the template rows yet.
			// We should copy template rows NOW, after horizontal expansion is done (if any),
			// because horizontal expansion might have widened the template row.
			if dataCount > 1 {
				// Calculate bounding box rows
				// Re-parse ranges because InsertCols might have shifted things?
				// InsertCols shifts columns to the right. It doesn't affect row indices.
				// BUT it affects the CONTENT of the row.
				// So yes, we should copy rows AFTER columns are expanded.

				_, startRow, _, endRow, err := parseRange(vH.Range.Ref)
				if err != nil {
					return err
				}
				axisHeight := endRow - startRow + 1
				insertCount := (dataCount - 1) * axisHeight

				if err := g.copyTemplateSlice(f, sheetName, block, hH.Name, endRow+1, insertCount, true); err != nil {
					return err
				}
			}
		}

	} else {
		// Horizontal Expand Mode
		headerData, err = g.Context.GetBlockDataWithParams(hH, params)
		if err != nil {
			return err
		}

		staticData, err = g.Context.GetBlockDataWithParams(vH, params)
		if err != nil {
			return err
		}

		if !allowExpansion && (len(headerData) > 1 || len(staticData) > 1) {
			return fmt.Errorf("nested matrix block %s cannot expand rows or columns", block.Name)
		}

		// Insert Cols logic
		if allowExpansion {
			dataCount := len(headerData)
			if dataCount > 1 {
				startCol, _, endCol, _, err := parseRange(hH.Range.Ref)
				if err != nil {
					return err
				}
				axisWidth := endCol - startCol + 1
				insertCount := (dataCount - 1) * axisWidth
				colName, _ := excelize.ColumnNumberToName(endCol + 1)
				if err := f.InsertCols(sheetName, colName, insertCount); err != nil {
					return err
				}

				// Copy Template Columns
				if err := g.copyTemplateSlice(f, sheetName, block, vH.Name, endCol+1, insertCount, false); err != nil {
					return err
				}
			}
		}
	}

	// In vertical expand mode: headerData=vH data, staticData=hH data.
	// In horizontal expand mode: headerData=hH data, staticData=vH data.
	// Resolve to the correct data for each axis before filling.
	vData, hData := headerData, staticData
	if !isVerticalExpand {
		vData, hData = staticData, headerData
	}

	// Fill vertical Headers
	if err := g.fillBlockData(f, sheetName, vH, vData); err != nil {
		return fmt.Errorf("failed to fill vertical headers: %w", err)
	}

	// Fill horizontal Headers
	if err := g.fillBlockData(f, sheetName, hH, hData); err != nil {
		return fmt.Errorf("failed to fill horizontal headers: %w", err)
	}

	// 5. Fill Intersection Data (Template Blocks)
	// Iterate over the grid defined by headerData x staticData
	// For each cell in the grid, find the corresponding TemplateBlock and fill it.

	// Collect Template Blocks (SubBlocks that are NOT Header)
	var templateBlocks []*config.BlockConfig
	var matrixBlocks []*config.BlockConfig
	for i := range block.SubBlocks {
		sb := &block.SubBlocks[i]
		// Use Template flag if available, otherwise fallback to Type != Header
		if sb.Template || sb.Type != config.BlockTypeHeader {
			if sb.Type == config.BlockTypeMatrix {
				matrixBlocks = append(matrixBlocks, sb)
			} else {
				templateBlocks = append(templateBlocks, sb)
			}
		}
	}

	// Pre-cache Template Content (Read-Once)
	var cachedTemplates []*TemplateCache
	for _, tb := range templateBlocks {
		cache, err := g.captureTemplate(f, sheetName, tb)
		if err != nil {
			return err
		}
		cachedTemplates = append(cachedTemplates, cache)
	}

	rows := headerData
	cols := staticData
	if !isVerticalExpand {
		rows = staticData
		cols = headerData
	}

	// ... (Header Param Key Logic) ...
	getHeaderParamKey := func(header *config.BlockConfig) (string, error) {
		if header.LabelVariable != "" {
			return header.LabelVariable, nil
		}
		conf, err := g.Context.ConfigProvider.GetDataViewConfig(header.DataViewName)
		if err != nil {
			return "", err
		}
		if len(conf.Labels) > 0 {
			return conf.Labels[0].Name, nil
		}
		return "", fmt.Errorf("cannot determine parameter key for header %s", header.Name)
	}

	vKey, err := getHeaderParamKey(vH)
	if err != nil {
		return err
	}
	hKey, err := getHeaderParamKey(hH)
	if err != nil {
		return err
	}

	// Reparse ranges to get dimensions for stepping
	_, vStartRow, _, vEndRow, _ := parseRange(vH.Range.Ref)
	vStep := vEndRow - vStartRow + 1

	hStartCol, _, hEndCol, _, _ := parseRange(hH.Range.Ref)
	hStep := hEndCol - hStartCol + 1

	// Iterate Grid & Fill (Write-Many)
	for r, rowItem := range rows {
		for c, colItem := range cols {
			// Construct parameters for this cell
			cellParams := cloneParams(params)

			// Resolve Label Name -> Column Name first!
			getLabelName := func(dataViewName, labelName string) string {
				conf, err := g.Context.ConfigProvider.GetDataViewConfig(dataViewName)
				if err != nil {
					return ""
				}
				for _, t := range conf.Labels {
					if t.Name == labelName {
						return t.Column
					}
				}
				return ""
			}

			vCol := getLabelName(vH.DataViewName, vKey)
			if vCol != "" {
				if val, ok := rowItem[vCol]; ok {
					cellParams[vKey] = fmt.Sprintf("%v", val)
				}
			}

			hCol := getLabelName(hH.DataViewName, hKey)
			if hCol != "" {
				if val, ok := colItem[hCol]; ok {
					cellParams[hKey] = fmt.Sprintf("%v", val)
				}
			}

			// Calculate Offsets
			rowOffset := r * vStep
			colOffset := c * hStep

			for _, mb := range matrixBlocks {
				offsetBlock, err := offsetBlockConfig(*mb, colOffset, rowOffset)
				if err != nil {
					return err
				}
				if err := g.processMatrixBlockWithParams(f, sheetName, &offsetBlock, cellParams, false); err != nil {
					return err
				}
			}

			// Process each Template using Cache
			for _, cache := range cachedTemplates {
				// Fetch Data for this cell
				cellDataList, err := g.Context.GetBlockDataWithParams(cache.Block, cellParams)
				if err != nil {
					return err
				}

				var dataRow map[string]interface{}
				if len(cellDataList) > 0 {
					dataRow = cellDataList[0]
				}

				// Fill Cells
				targetC := cache.StartCol + colOffset
				targetR := cache.StartRow + rowOffset
				if err := g.fillTemplate(f, sheetName, cache, targetC, targetR, dataRow); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *Generator) copyTemplateSlice(f ExcelFile, sheetName string, block *config.BlockConfig, skipBlockName string, destStart, insertCount int, isRowMode bool) error {
	minVal, maxVal := math.MaxInt, -1
	for i := range block.SubBlocks {
		sb := &block.SubBlocks[i]
		if sb.Name == skipBlockName {
			continue
		}
		c1, r1, c2, r2, err := parseRange(sb.Range.Ref)
		if err == nil {
			var v1, v2 int
			if isRowMode {
				v1, v2 = r1, r2
			} else {
				v1, v2 = c1, c2
			}
			if v1 < minVal {
				minVal = v1
			}
			if v2 > maxVal {
				maxVal = v2
			}
		}
	}

	if minVal <= maxVal {
		if err := g.copySlice(f, sheetName, isRowMode, minVal, maxVal, destStart, insertCount); err != nil {
			return fmt.Errorf("failed to copy template slice: %w", err)
		}
	}
	return nil
}

// copySlice copies a range of rows or columns to a new location.
// isRowMode: true for copying rows (vertical shift), false for copying columns (horizontal shift).
// srcStart, srcEnd: range of rows (if isRowMode) or columns (if !isRowMode) to copy.
// destStart: starting row (if isRowMode) or column (if !isRowMode) to paste.
// count: number of rows/columns to paste (total destination size).
func (g *Generator) copySlice(f ExcelFile, sheet string, isRowMode bool, srcStart, srcEnd, destStart, count int) error {
	srcSize := srcEnd - srcStart + 1
	type cellData struct {
		val   string
		style int
	}
	srcMap := make(map[int]cellData)

	// Determine limits
	dims, err := f.GetSheetDimension(sheet)
	if err != nil {
		return err
	}
	_, _, maxC, maxR, err := parseRange(dims)
	if err != nil {
		maxC, maxR = 100, 1000 // Fallback
	}

	limit := maxC
	if !isRowMode {
		limit = maxR
	}

	// 1. Read Source
	for p := srcStart; p <= srcEnd; p++ { // Primary loop (row if isRowMode, col if !isRowMode)
		for s := 1; s <= limit; s++ { // Secondary loop (col if isRowMode, row if !isRowMode)
			var c, r int
			if isRowMode {
				c, r = s, p
			} else {
				c, r = p, s
			}
			cn, _ := excelize.CoordinatesToCellName(c, r)
			val, _ := f.GetCellValue(sheet, cn)
			style, _ := f.GetCellStyle(sheet, cn)
			// Key: (PrimaryOffset * 10000) + SecondaryIndex
			key := (p-srcStart)*10000 + s
			srcMap[key] = cellData{val, style}
		}
	}

	// 2. Write to Dest
	for i := range count {
		srcOffset := i % srcSize
		destP := destStart + i

		for s := 1; s <= limit; s++ {
			key := srcOffset*10000 + s
			if data, ok := srcMap[key]; ok {
				var c, r int
				if isRowMode {
					c, r = s, destP
				} else {
					c, r = destP, s
				}
				cn, _ := excelize.CoordinatesToCellName(c, r)
				_ = f.SetCellValue(sheet, cn, data.val)
				if data.style != 0 {
					_ = f.SetCellStyle(sheet, cn, cn, data.style)
				}
			}
		}
	}
	return nil
}

// copyRows copies a range of rows to a new location, replicating them count times.
func (g *Generator) copyRows(f ExcelFile, sheet string, srcStartRow, srcEndRow, destStartRow, insertHeight int) error {
	return g.copySlice(f, sheet, true, srcStartRow, srcEndRow, destStartRow, insertHeight)
}

// copyCols copies a range of columns to a new location.
func (g *Generator) copyCols(f ExcelFile, sheet string, srcStartCol, srcEndCol, destStartCol, insertWidth int) error {
	return g.copySlice(f, sheet, false, srcStartCol, srcEndCol, destStartCol, insertWidth)
}

func (g *Generator) processValueBlock(f ExcelFile, sheetName string, block *config.BlockConfig) error {
	return g.processValueBlockWithParams(f, sheetName, block, g.Context.Parameters)
}

func (g *Generator) processValueBlockWithParams(f ExcelFile, sheetName string, block *config.BlockConfig, params map[string]string) error {
	data, err := g.Context.GetBlockDataWithParams(block, params)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil // No data
	}

	// Determine direction (default: vertical)
	isVertical := block.Direction == config.DirectionVertical || block.Direction == ""

	// Parse Template Range
	startCol, startRow, endCol, endRow, err := parseRange(block.Range.Ref)
	if err != nil {
		return err
	}

	// Height/Width of the template block
	blockHeight := endRow - startRow + 1
	blockWidth := endCol - startCol + 1

	// 1. Expand (Insert Rows/Columns) if needed
	// Note: In C#, expansion happens only if block.Expand is true (which maps to InsertAfter here probably, or explicit Expand flag)
	// But ValueBlock in C# also has an implicit loop if multiple rows are returned.
	// Let's assume we always expand if row count > 1, similar to C# logic.
	dataCount := len(data)
	if dataCount > 1 {
		// Calculate how many new rows/cols we need.
		// We already have 1 set of rows/cols in the template. We need (dataCount - 1) more.
		if isVertical {
			insertCount := (dataCount - 1) * blockHeight
			// Insert after the block's bottom
			if err := f.InsertRows(sheetName, endRow+1, insertCount); err != nil {
				return fmt.Errorf("failed to insert rows: %w", err)
			}
		} else {
			insertCount := (dataCount - 1) * blockWidth
			// Insert after the block's right
			// Excelize InsertCols takes column name "C"
			colName, _ := excelize.ColumnNumberToName(endCol + 1)
			if err := f.InsertCols(sheetName, colName, insertCount); err != nil {
				return fmt.Errorf("failed to insert cols: %w", err)
			}
		}
	}

	// 2. Fill Data
	return g.fillBlockData(f, sheetName, block, data)
}

// CellData fillBlockData fills a block with data, handling template caching and label replacement.
// It assumes any necessary expansion (inserting rows/cols) has already been done.
type CellData struct {
	Val   string
	Style int
}

type RelativeMerge struct {
	StartCol, StartRow, EndCol, EndRow int
}

type TemplateCache struct {
	Block    *config.BlockConfig
	Cells    [][]CellData // [row][col]
	Merged   []RelativeMerge
	StartCol int
	StartRow int
	Width    int
	Height   int
}

func (g *Generator) captureTemplate(f ExcelFile, sheetName string, block *config.BlockConfig) (*TemplateCache, error) {
	c1, r1, c2, r2, err := parseRange(block.Range.Ref)
	if err != nil {
		return nil, err
	}
	w := c2 - c1 + 1
	h := r2 - r1 + 1
	cells := make([][]CellData, h)
	for r := range h {
		cells[r] = make([]CellData, w)
		for c := range w {
			cn, _ := excelize.CoordinatesToCellName(c1+c, r1+r)
			val, _ := f.GetCellValue(sheetName, cn)
			sty, _ := f.GetCellStyle(sheetName, cn)
			cells[r][c] = CellData{Val: val, Style: sty}
		}
	}

	// Capture Merged Cells
	var relativeMerges []RelativeMerge
	mergedCells, err := f.GetMergeCells(sheetName)
	if err == nil {
		for _, mc := range mergedCells {
			mcStart, mcEnd := mc.GetStartAxis(), mc.GetEndAxis()
			mcC1, mcR1, err := excelize.CellNameToCoordinates(mcStart)
			if err != nil {
				continue
			}
			mcC2, mcR2, err := excelize.CellNameToCoordinates(mcEnd)
			if err != nil {
				continue
			}

			// Check if merge is fully contained in the block
			if mcC1 >= c1 && mcR1 >= r1 && mcC2 <= c2 && mcR2 <= r2 {
				relativeMerges = append(relativeMerges, RelativeMerge{
					StartCol: mcC1 - c1,
					StartRow: mcR1 - r1,
					EndCol:   mcC2 - c1,
					EndRow:   mcR2 - r1,
				})
			}
		}
	}

	return &TemplateCache{
		Block:    block,
		Cells:    cells,
		Merged:   relativeMerges,
		StartCol: c1,
		StartRow: r1,
		Width:    w,
		Height:   h,
	}, nil
}

func (g *Generator) fillTemplate(f ExcelFile, sheetName string, cache *TemplateCache, targetCol, targetRow int, data map[string]interface{}) error {
	// Replacement map
	rep := make(map[string]interface{})
	if data != nil {
		vv, err := g.Context.ConfigProvider.GetDataViewConfig(cache.Block.DataViewName)
		if err == nil {
			for _, t := range vv.Labels {
				if v, ok := data[t.Column]; ok {
					rep[t.Name] = v
				}
			}
		}
	}

	for r := 0; r < cache.Height; r++ {
		for c := 0; c < cache.Width; c++ {
			cell := cache.Cells[r][c]
			val := cell.Val
			style := cell.Style

			// Replace
			for t, v := range rep {
				ph := fmt.Sprintf("{%s}", t)
				if strings.Contains(val, ph) {
					val = strings.ReplaceAll(val, ph, fmt.Sprintf("%v", v))
				}
			}

			tcn, _ := excelize.CoordinatesToCellName(targetCol+c, targetRow+r)
			if err := f.SetCellValue(sheetName, tcn, val); err != nil {
				return err
			}
			if style != 0 {
				if err := f.SetCellStyle(sheetName, tcn, tcn, style); err != nil {
					return err
				}
			}
		}
	}

	// Apply Merged Cells
	for _, rm := range cache.Merged {
		c1, r1 := targetCol+rm.StartCol, targetRow+rm.StartRow
		c2, r2 := targetCol+rm.EndCol, targetRow+rm.EndRow

		startCell, err := excelize.CoordinatesToCellName(c1, r1)
		if err != nil {
			return err
		}
		endCell, err := excelize.CoordinatesToCellName(c2, r2)
		if err != nil {
			return err
		}

		if err := f.MergeCell(sheetName, startCell, endCell); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) fillBlockData(f ExcelFile, sheetName string, block *config.BlockConfig, data []map[string]interface{}) error {
	// Capture Template
	cache, err := g.captureTemplate(f, sheetName, block)
	if err != nil {
		return err
	}

	isVert := block.Direction == config.DirectionVertical || block.Direction == ""

	// Iterate
	for i, row := range data {
		rOff, cOff := 0, 0
		if isVert {
			rOff = i * cache.Height
		} else {
			cOff = i * cache.Width
		}

		// Fill
		if err := g.fillTemplate(f, sheetName, cache, cache.StartCol+cOff, cache.StartRow+rOff, row); err != nil {
			return err
		}
	}
	return nil
}

// Helper to parse "A1:B2"
func parseRange(ref string) (int, int, int, int, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("invalid range: %s", ref)
	}
	c1, r1, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	c2, r2, err := excelize.CellNameToCoordinates(parts[1])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return c1, r1, c2, r2, nil
}

func offsetRange(ref string, colOffset, rowOffset int) (string, error) {
	c1, r1, c2, r2, err := parseRange(ref)
	if err != nil {
		return "", err
	}
	startCell, err := excelize.CoordinatesToCellName(c1+colOffset, r1+rowOffset)
	if err != nil {
		return "", err
	}
	endCell, err := excelize.CoordinatesToCellName(c2+colOffset, r2+rowOffset)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", startCell, endCell), nil
}

func offsetBlockConfig(block config.BlockConfig, colOffset, rowOffset int) (config.BlockConfig, error) {
	updated := block
	if block.Range.Ref != "" {
		ref, err := offsetRange(block.Range.Ref, colOffset, rowOffset)
		if err != nil {
			return config.BlockConfig{}, err
		}
		updated.Range.Ref = ref
	}
	if len(block.SubBlocks) > 0 {
		updated.SubBlocks = make([]config.BlockConfig, len(block.SubBlocks))
		for i, sub := range block.SubBlocks {
			offsetSub, err := offsetBlockConfig(sub, colOffset, rowOffset)
			if err != nil {
				return config.BlockConfig{}, err
			}
			updated.SubBlocks[i] = offsetSub
		}
	}
	return updated, nil
}
