package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/gin-gonic/gin"
)

func TestHandleUploadStoresOriginalNameAndReportsPaths(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	var stored struct {
		FileID       string
		RelativePath string
		StorePath    string
	}
	OnFileStored = func(event StoredFileEvent) {
		stored.FileID = event.FileID
		stored.RelativePath = event.RelativePath
		stored.StorePath = event.StorePath
	}
	defer func() { OnFileStored = nil }()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "file_id", Value: "file-123"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-123?path=/docs&name=report.txt", strings.NewReader("payload"))

	handleUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	expectedPath := filepath.Join(storage.FilesDir(), "docs", "report.txt")
	if resp["relative_path"] != "/docs/report.txt" {
		t.Fatalf("expected relative path /docs/report.txt, got %q", resp["relative_path"])
	}
	if resp["store_path"] != expectedPath {
		t.Fatalf("expected store path %q, got %q", expectedPath, resp["store_path"])
	}
	if stored.FileID != "file-123" || stored.RelativePath != "/docs/report.txt" || stored.StorePath != expectedPath {
		t.Fatalf("unexpected stored event: %#v", stored)
	}
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("expected payload, got %q", string(data))
	}
}

func TestHandleUploadRejectsExistingNameWithoutOverwrite(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	target := filepath.Join(storage.FilesDir(), "docs", "report.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "file_id", Value: "file-123"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-123?path=/docs&name=report.txt", strings.NewReader("payload"))

	handleUpload(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body=%s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("expected original data to stay, got %q", string(data))
	}
}

func TestHandleBrowseDownloadReturnsZipForDirectory(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	target := filepath.Join(storage.FilesDir(), "bundle", "hello.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/browse?path=/bundle&filename=bundle.zip", nil)

	handleBrowseDownload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	readerAt := bytes.NewReader(w.Body.Bytes())
	zr, err := zip.NewReader(readerAt, int64(readerAt.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("expected 1 file in zip, got %d", len(zr.File))
	}
	if zr.File[0].Name != "hello.txt" {
		t.Fatalf("expected hello.txt in zip, got %q", zr.File[0].Name)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open zipped file: %v", err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zipped file: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("expected hello, got %q", string(body))
	}
}

func TestValidateSignedURLRejectsTamperedUploadQuery(t *testing.T) {
	prevSecret := signingSecret
	signingSecret = "test-secret"
	defer func() { signingSecret = prevSecret }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/files/:file_id", validateSignedURL(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	expires := time.Now().Add(time.Minute).Unix()
	fileID := "file-123"
	sig := signResource(signedUploadResource(fileID, "/docs", "report.txt", "false"), http.MethodPut, expires)

	okReq := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/files/%s?path=%s&name=%s&overwrite=false&expires=%d&sig=%s", fileID, url.QueryEscape("/docs"), url.QueryEscape("report.txt"), expires, sig),
		nil,
	)
	okResp := httptest.NewRecorder()
	r.ServeHTTP(okResp, okReq)
	if okResp.Code != http.StatusNoContent {
		t.Fatalf("expected untampered request to pass, got %d body=%s", okResp.Code, okResp.Body.String())
	}

	tamperedReq := httptest.NewRequest(
		http.MethodPut,
		fmt.Sprintf("/api/files/%s?path=%s&name=%s&overwrite=true&expires=%d&sig=%s", fileID, url.QueryEscape("/docs"), url.QueryEscape("report.txt"), expires, sig),
		nil,
	)
	tamperedResp := httptest.NewRecorder()
	r.ServeHTTP(tamperedResp, tamperedReq)
	if tamperedResp.Code != http.StatusForbidden {
		t.Fatalf("expected tampered request to be rejected, got %d body=%s", tamperedResp.Code, tamperedResp.Body.String())
	}
}

func withTempStorageRoot(t *testing.T) func() {
	t.Helper()
	root := t.TempDir()
	prev := os.Getenv("CLOUDNEST_DATA_SAVE_DIR")
	if err := os.Setenv("CLOUDNEST_DATA_SAVE_DIR", root); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if err := storage.EnsureDataDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	return func() {
		if prev == "" {
			_ = os.Unsetenv("CLOUDNEST_DATA_SAVE_DIR")
			return
		}
		_ = os.Setenv("CLOUDNEST_DATA_SAVE_DIR", prev)
	}
}
