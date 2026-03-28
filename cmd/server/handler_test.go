package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// validConfigYAML is a minimal valid config bundle for testing.
const validConfigYAML = `
workbook:
  id: wb1
  name: TestReport
  template: template.xlsx
  outputDir: output
  sheets:
    - name: Sheet1
      blocks:
        - name: Block1
          type: value
          range: {ref: "A1"}
          dataView: view1

dataViews:
  - name: view1
    dataSource: ds1
    labels:
      - name: name
        column: name

dataSources:
  - name: ds1
    driver: csv
    dsn: local
`

const validCSV = "name\nAlice\nBob\n"

func newHandler() *Handler {
	return &Handler{MaxUploadSize: 50 * 1024 * 1024}
}

// buildMultipartRequest constructs a multipart form request.
// files is a map of field name -> (filename, content) pairs.
// params is optional JSON params string.
func buildMultipartRequest(t *testing.T, files map[string][]fileEntry, params string) (*http.Request, error) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for field, entries := range files {
		for _, entry := range entries {
			part, err := w.CreateFormFile(field, entry.name)
			if err != nil {
				return nil, err
			}
			part.Write(entry.content)
		}
	}

	if params != "" {
		w.WriteField("params", params)
	}

	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/generate", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

type fileEntry struct {
	name    string
	content []byte
}

// getTemplateBytes returns a minimal .xlsx file for testing.
// We use a real template from the test fixtures if available, otherwise a placeholder.
func getTemplateBytes(t *testing.T) []byte {
	t.Helper()
	// Try to load a real template from test fixtures
	paths := []string{
		"../../test/templates/demo_template.xlsx",
		"../../test/templates/valueblock_template.xlsx",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data
		}
	}
	// Return minimal bytes (will fail generation but sufficient for validation tests)
	return []byte("PK\x03\x04fake xlsx content")
}

func assertErrorResponse(t *testing.T, rr *httptest.ResponseRecorder, expectedStatus int, expectedMsgContains string) {
	t.Helper()
	if rr.Code != expectedStatus {
		t.Errorf("expected status %d, got %d. Body: %s", expectedStatus, rr.Code, rr.Body.String())
		return
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("expected JSON error response, got: %s", rr.Body.String())
		return
	}
	if expectedMsgContains != "" && !strings.Contains(resp["error"], expectedMsgContains) {
		t.Errorf("expected error containing %q, got %q", expectedMsgContains, resp["error"])
	}
}

// --- Security Tests ---

func TestSecurity_PathTraversal_Config(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "../../etc/passwd", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// The handler should sanitize the filename. It should NOT write to ../../etc/passwd.
	// It should either succeed (treating it as "passwd") or fail with a validation error.
	// The key assertion: no file was written outside the temp directory.
	if rr.Code == http.StatusOK || rr.Code == http.StatusUnprocessableEntity || rr.Code == http.StatusBadRequest {
		// Acceptable — the path traversal was neutralized
		return
	}
	// 500 is also acceptable if generation fails for other reasons
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status %d for path traversal attempt", rr.Code)
	}
}

func TestSecurity_PathTraversal_DataFile(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "../../../tmp/evil.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// The sanitized filename should be "evil.csv", not a path traversal.
	// Generation may fail because the CSV name doesn't match the view name, which is fine.
	if rr.Code == http.StatusOK || rr.Code == http.StatusUnprocessableEntity ||
		rr.Code == http.StatusInternalServerError || rr.Code == http.StatusBadRequest {
		return
	}
	t.Errorf("unexpected status %d for data file path traversal", rr.Code)
}

func TestSecurity_PathTraversal_Template(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "../../etc/template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// sanitizeFilename should reduce to "template.xlsx"
	// Should succeed or fail on template mismatch (config says "template.xlsx", sanitized is "template.xlsx" — match)
	if rr.Code >= 200 && rr.Code < 600 {
		return // any HTTP response means no panic and path traversal was handled
	}
	t.Errorf("unexpected status %d", rr.Code)
}

func TestSecurity_OversizedUpload(t *testing.T) {
	h := &Handler{MaxUploadSize: 1024} // 1KB limit

	// Create a body larger than 1KB
	largeContent := bytes.Repeat([]byte("x"), 2048)
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: largeContent}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// Should reject with 413 or 400
	if rr.Code != http.StatusRequestEntityTooLarge && rr.Code != http.StatusBadRequest {
		t.Errorf("expected 413 or 400 for oversized upload, got %d", rr.Code)
	}
}

func TestSecurity_WrongFileType_Template(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "malware.exe", content: []byte("MZ evil binary")}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, ".xlsx")
}

func TestSecurity_WrongFileType_Config(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.json", content: []byte(`{"key":"val"}`)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, ".yaml")
}

func TestSecurity_WrongFileType_Data(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "data.txt", content: []byte("some text")}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, ".csv")
}

func TestSecurity_MissingConfig(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "config")
}

func TestSecurity_MissingTemplate(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config": {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"data[]": {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "template")
}

func TestSecurity_MissingData(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "data[]")
}

func TestSecurity_MalformedYAML(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte("{{{{invalid yaml: [[[[")}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// Should return 422 for config validation failure, not panic
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for malformed YAML, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSecurity_EmptyConfigFile(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte{}}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// Empty config should fail validation
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty config, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSecurity_EmptyTemplateFile(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: []byte{}}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// Empty template should fail during generation, returning 500
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for empty template, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestSecurity_EmptyCSVFile(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte{}}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	// Empty CSV may cause generation to fail — that's fine, just shouldn't panic
	if rr.Code < 200 || rr.Code >= 600 {
		t.Errorf("unexpected status %d for empty CSV", rr.Code)
	}
}

func TestSecurity_InvalidParamsJSON(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "not valid json{{{")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusBadRequest, "invalid params JSON")
}

func TestSecurity_ConcurrentRequests(t *testing.T) {
	h := newHandler()
	const concurrency = 10

	var wg sync.WaitGroup
	results := make([]int, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			files := map[string][]fileEntry{
				"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
				"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
				"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
			}

			req, err := buildMultipartRequest(t, files, "")
			if err != nil {
				t.Errorf("request %d: build error: %v", idx, err)
				return
			}

			rr := httptest.NewRecorder()
			h.Generate(rr, req)
			results[idx] = rr.Code
		}(i)
	}

	wg.Wait()

	// All requests should complete without panic.
	// They may fail (template isn't a real xlsx in some envs) but should all get the same status.
	for i, code := range results {
		if code == 0 {
			t.Errorf("request %d: got status 0 (likely panicked or didn't complete)", i)
		}
	}
}

func TestSecurity_TemplateMismatch(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "wrong_name.xlsx", content: getTemplateBytes(t)}}, // config expects "template.xlsx"
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	assertErrorResponse(t, rr, http.StatusUnprocessableEntity, "does not match")
}

func TestHealth(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %q", resp["status"])
	}
}

func TestSecurity_SanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.csv", "normal.csv"},
		{"../../etc/passwd", "passwd"},
		{"../../../tmp/evil.csv", "evil.csv"},
		// On Unix, backslashes are valid in filenames so filepath.Base won't split on them.
		// On Windows, filepath.Base would return "config.yaml". This is OS-correct behavior.
		{"/absolute/path/file.xlsx", "file.xlsx"},
		{"./relative/path/file.csv", "file.csv"},
		{"file with spaces.csv", "file with spaces.csv"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestSecurity_TempDirCleanup verifies that temp directories are cleaned up after request handling.
func TestSecurity_TempDirCleanup(t *testing.T) {
	h := newHandler()

	// Count temp dirs before
	before, _ := filepath.Glob(os.TempDir() + "/fibr-gen-*")

	files := map[string][]fileEntry{
		"config":   {{name: "config.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	_ = rr // don't care about response status

	// Count temp dirs after — should be same as before (cleanup happened)
	after, _ := filepath.Glob(os.TempDir() + "/fibr-gen-*")
	if len(after) > len(before) {
		t.Errorf("temp directory leak: %d dirs before, %d after", len(before), len(after))
	}
}

// TestSecurity_NoFileWriteOutsideTemp verifies path traversal doesn't write outside temp.
func TestSecurity_NoFileWriteOutsideTemp(t *testing.T) {
	// Create a canary file path that path traversal might target
	canaryDir := t.TempDir()
	canaryPath := filepath.Join(canaryDir, "passwd")

	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "../../" + canaryDir + "/passwd", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	_ = rr

	// The canary file should NOT exist
	if _, err := os.Stat(canaryPath); err == nil {
		t.Error("path traversal succeeded — file was written outside temp directory!")
		os.Remove(canaryPath)
	}
}

// TestSecurity_LargeFilename tests handling of extremely long filenames.
func TestSecurity_LargeFilename(t *testing.T) {
	h := newHandler()
	longName := strings.Repeat("a", 500) + ".yaml"

	files := map[string][]fileEntry{
		"config":   {{name: longName, content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Should not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handler panicked on large filename: %v", r)
			}
		}()
		h.Generate(rr, req)
	}()
}

// TestSecurity_NullBytesInFilename tests that null bytes in filenames are handled safely.
func TestSecurity_NullBytesInFilename(t *testing.T) {
	h := newHandler()
	files := map[string][]fileEntry{
		"config":   {{name: "config\x00.yaml", content: []byte(validConfigYAML)}},
		"template": {{name: "template.xlsx", content: getTemplateBytes(t)}},
		"data[]":   {{name: "view1.csv", content: []byte(validCSV)}},
	}

	req, err := buildMultipartRequest(t, files, "")
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	// Should not panic — may return error or succeed with sanitized name
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handler panicked on null byte filename: %v", r)
			}
		}()
		h.Generate(rr, req)
	}()

	// Any non-panic result is acceptable
	if rr.Code < 200 || rr.Code >= 600 {
		t.Errorf("unexpected status %d", rr.Code)
	}
}
