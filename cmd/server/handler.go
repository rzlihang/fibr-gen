package main

import (
	"encoding/json"
	"fibr-gen/config"
	"fibr-gen/core"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Handler struct {
	MaxUploadSize int64
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds maximum size")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	// Validate required files
	configFile, configHeader, err := r.FormFile("config")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing required field: config")
		return
	}
	defer configFile.Close()

	templateFile, templateHeader, err := r.FormFile("template")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing required field: template")
		return
	}
	defer templateFile.Close()

	dataFiles := r.MultipartForm.File["data[]"]
	if len(dataFiles) == 0 {
		writeError(w, http.StatusBadRequest, "missing required field: data[] (at least one CSV file)")
		return
	}

	// Validate file extensions
	configName := sanitizeFilename(configHeader.Filename)
	ext := strings.ToLower(filepath.Ext(configName))
	if ext != ".yaml" && ext != ".yml" {
		writeError(w, http.StatusBadRequest, "config file must be .yaml or .yml")
		return
	}

	templateName := sanitizeFilename(templateHeader.Filename)
	if strings.ToLower(filepath.Ext(templateName)) != ".xlsx" {
		writeError(w, http.StatusBadRequest, "template file must be .xlsx")
		return
	}

	for _, fh := range dataFiles {
		name := sanitizeFilename(fh.Filename)
		if strings.ToLower(filepath.Ext(name)) != ".csv" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("data file %q must be .csv", fh.Filename))
			return
		}
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fibr-gen-*")
	if err != nil {
		slog.Error("failed to create temp dir", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	templatesDir := filepath.Join(tmpDir, "templates")
	dataDir := filepath.Join(tmpDir, "data")
	outputDir := filepath.Join(tmpDir, "output")
	for _, d := range []string{templatesDir, dataDir, outputDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			slog.Error("failed to create dir", "path", d, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	// Write config file
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := saveUpload(configFile, configPath); err != nil {
		slog.Error("failed to save config", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Write template file
	templatePath := filepath.Join(templatesDir, templateName)
	if err := saveUpload(templateFile, templatePath); err != nil {
		slog.Error("failed to save template", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Write data CSV files
	for _, fh := range dataFiles {
		name := sanitizeFilename(fh.Filename)
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read data file %q", fh.Filename))
			return
		}
		if err := saveUpload(f, filepath.Join(dataDir, name)); err != nil {
			f.Close()
			slog.Error("failed to save data file", "name", name, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		f.Close()
	}

	// Parse optional params
	params := make(map[string]string)
	if paramsJSON := r.FormValue("params"); paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			writeError(w, http.StatusBadRequest, "invalid params JSON: "+err.Error())
			return
		}
	}

	// Load config bundle
	wbConf, views, dataSources, err := config.LoadConfigBundle(configPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "config validation failed: "+err.Error())
		return
	}

	// Validate template filename matches config
	if wbConf.Template != templateName {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("uploaded template filename %q does not match config workbook.template %q", templateName, wbConf.Template))
		return
	}

	// Create CSV fetcher pointed at uploaded data
	fetcher := core.NewCsvDataFetcher(dataDir)

	// Create config registry and generation context
	registry := config.NewMemoryConfigRegistry(views, dataSources)
	ctx := core.NewGenerationContext(wbConf, registry, fetcher, params)

	// Run generation
	generator := core.NewGenerator(ctx)
	if err := generator.Generate(templatesDir, outputDir); err != nil {
		slog.Error("generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "report generation failed: "+err.Error())
		return
	}

	// Find the output file
	outputFile, err := findOutputXLSX(outputDir)
	if err != nil {
		slog.Error("output file not found", "error", err)
		writeError(w, http.StatusInternalServerError, "generated file not found")
		return
	}

	// Stream the output file
	f, err := os.Open(outputFile)
	if err != nil {
		slog.Error("failed to open output", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer f.Close()

	outputName := filepath.Base(outputFile)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, outputName))
	io.Copy(w, f)
}

type ParsedSheet struct {
	Name   string     `json:"name"`
	Rows   [][]string `json:"rows"`
	MaxRow int        `json:"maxRow"`
	MaxCol int        `json:"maxCol"`
}

type ParseTemplateResponse struct {
	Sheets []ParsedSheet `json:"sheets"`
}

const maxParseRows = 100
const maxParseCols = 50

func (h *Handler) ParseTemplate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxUploadSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds maximum size")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("template")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing required field: template")
		return
	}
	defer file.Close()

	name := sanitizeFilename(header.Filename)
	if strings.ToLower(filepath.Ext(name)) != ".xlsx" {
		writeError(w, http.StatusBadRequest, "template file must be .xlsx")
		return
	}

	tmpDir, err := os.MkdirTemp("", "fibr-parse-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, name)
	if err := saveUpload(file, tmpPath); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	f, err := excelize.OpenFile(tmpPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to open xlsx file: "+err.Error())
		return
	}
	defer f.Close()

	var sheets []ParsedSheet
	for _, sheetName := range f.GetSheetList() {
		allRows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}

		maxRow := len(allRows)
		if maxRow > maxParseRows {
			maxRow = maxParseRows
		}

		maxCol := 0
		for _, row := range allRows[:maxRow] {
			if len(row) > maxCol {
				maxCol = len(row)
			}
		}
		if maxCol > maxParseCols {
			maxCol = maxParseCols
		}

		rows := make([][]string, maxRow)
		for i, row := range allRows[:maxRow] {
			cols := make([]string, maxCol)
			for j := 0; j < maxCol && j < len(row); j++ {
				cols[j] = row[j]
			}
			rows[i] = cols
		}

		sheets = append(sheets, ParsedSheet{
			Name:   sheetName,
			Rows:   rows,
			MaxRow: maxRow,
			MaxCol: maxCol,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ParseTemplateResponse{Sheets: sheets})
}

// sanitizeFilename strips directory components to prevent path traversal.
func sanitizeFilename(name string) string {
	// filepath.Base handles both / and \ separators
	return filepath.Base(name)
}

func saveUpload(src multipart.File, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func findOutputXLSX(root string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".xlsx" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no .xlsx file found in output directory")
	}
	return found, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
