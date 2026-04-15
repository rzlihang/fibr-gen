package core

import (
	"fibr-gen/config"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// makeTemplateFile creates a minimal xlsx in dir with the given sheet name and optional cell value.
func makeTemplateFile(t *testing.T, dir, filename, sheetName, cell, value string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
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

func TestTemplateValidator_ValidConfig_NoErrors(t *testing.T) {
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
