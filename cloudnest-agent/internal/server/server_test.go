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
	"github.com/cloudnest/cloudnest-agent/internal/testutil"
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

	var resp struct {
		RelativePath string `json:"relative_path"`
		StorePath    string `json:"store_path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	expectedPath := filepath.Join(storage.FilesDir(), "docs", "report.txt")
	if resp.RelativePath != "/docs/report.txt" {
		t.Fatalf("expected relative path /docs/report.txt, got %q", resp.RelativePath)
	}
	if resp.StorePath != expectedPath {
		t.Fatalf("expected store path %q, got %q", expectedPath, resp.StorePath)
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

func TestHandleUploadOverwriteKeepsOriginalFileOnWriteFailure(t *testing.T) {
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
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-123?path=/docs&name=report.txt&overwrite=true", &failingUploadBody{
		chunks: [][]byte{[]byte("new")},
		err:    io.ErrUnexpectedEOF,
	})

	handleUpload(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d body=%s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("expected original data to stay untouched, got %q", string(data))
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

func TestHandleUploadRejectsSymlinkDirectory(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	link := filepath.Join(storage.FilesDir(), "shared")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "file_id", Value: "file-123"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-123?path=/shared&name=report.txt", strings.NewReader("payload"))

	handleUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleUploadRejectsSymlinkTarget(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	docsDir := filepath.Join(storage.FilesDir(), "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	link := filepath.Join(docsDir, "report.txt")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "file_id", Value: "file-123"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-123?path=/docs&name=report.txt&overwrite=true", strings.NewReader("payload"))

	handleUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleBrowseDownloadRejectsSymlinkPath(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	link := filepath.Join(storage.FilesDir(), "report.txt")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/browse?path=/report.txt", nil)

	handleBrowseDownload(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleBrowseDownloadSkipsSymlinkEntriesInZip(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	bundleDir := filepath.Join(storage.FilesDir(), "bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	testutil.CreateSymlinkOrSkip(t, outside, filepath.Join(bundleDir, "linked.txt"))

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
		t.Fatalf("expected only real files in zip, got %d entries", len(zr.File))
	}
	if zr.File[0].Name != "hello.txt" {
		t.Fatalf("expected hello.txt in zip, got %q", zr.File[0].Name)
	}
}

func TestHandleMoveRenamesManagedFile(t *testing.T) {
	restore := withTempStorageRoot(t)
	defer restore()

	source := filepath.Join(storage.FilesDir(), "docs", "report.txt")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(source, []byte("payload"), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/files/move?from=%2Fdocs%2Freport.txt&to=%2Farchive%2Freport-2026.txt", nil)

	handleMove(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	target := filepath.Join(storage.FilesDir(), "archive", "report-2026.txt")
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source to be moved away, stat err=%v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read moved target: %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("expected moved file contents to match, got %q", string(data))
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

func TestLoadSigningSecretFromEnvRejectsMissingValue(t *testing.T) {
	prevSecret := signingSecret
	signingSecret = ""
	t.Setenv("CLOUDNEST_SIGNING_SECRET", "")
	defer func() { signingSecret = prevSecret }()

	if err := LoadSigningSecretFromEnv(); err == nil {
		t.Fatal("expected missing signing secret to be rejected")
	}
}

func TestLoadSigningSecretFromEnvRejectsLegacyDefault(t *testing.T) {
	prevSecret := signingSecret
	signingSecret = ""
	t.Setenv("CLOUDNEST_SIGNING_SECRET", "cloudnest-default-secret")
	defer func() { signingSecret = prevSecret }()

	if err := LoadSigningSecretFromEnv(); err == nil {
		t.Fatal("expected legacy default signing secret to be rejected")
	}
}

func TestLoadSigningSecretFromEnvUsesConfiguredValue(t *testing.T) {
	prevSecret := signingSecret
	signingSecret = ""
	t.Setenv("CLOUDNEST_SIGNING_SECRET", "agent-secret")
	defer func() { signingSecret = prevSecret }()

	if err := LoadSigningSecretFromEnv(); err != nil {
		t.Fatalf("load signing secret: %v", err)
	}
	if signingSecret != "agent-secret" {
		t.Fatalf("expected signing secret to be updated, got %q", signingSecret)
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

type failingUploadBody struct {
	chunks [][]byte
	err    error
	index  int
}

func (r *failingUploadBody) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		if r.err == nil {
			return 0, io.EOF
		}
		return 0, r.err
	}
	chunk := r.chunks[r.index]
	r.index++
	n := copy(p, chunk)
	if r.index >= len(r.chunks) && r.err != nil {
		return n, r.err
	}
	return n, nil
}
