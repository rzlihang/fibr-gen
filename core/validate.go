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

// scanPlaceholders is completed in Task 4. Stub returns no issues.
func (v *TemplateValidator) scanPlaceholders(f *excelize.File, sheetName, blockName, ref string, labelToViews map[string][]string, usedLabels map[string]bool) []ValidationIssue {
	return nil
}
