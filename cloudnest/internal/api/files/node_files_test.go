package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	agentapi "github.com/cloudnest/cloudnest/internal/api/agent"
	"github.com/cloudnest/cloudnest/internal/cache"
	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/gin-gonic/gin"
)

var nodeFilesTestDBOnce sync.Once

func initNodeFilesTestDB(t *testing.T) {
	t.Helper()

	nodeFilesTestDBOnce.Do(func() {
		gin.SetMode(gin.TestMode)
		cache.Init()

		dsn := filepath.Join(os.TempDir(), fmt.Sprintf("cloudnest-node-files-%d.db", os.Getpid()))
		_ = os.Remove(dsn)
		if err := dbcore.Init("sqlite", dsn); err != nil {
			t.Fatalf("init db: %v", err)
		}
	})

	db := dbcore.DB()
	if err := db.Exec("DELETE FROM file_replicas").Error; err != nil {
		t.Fatalf("clear file_replicas: %v", err)
	}
	if err := db.Exec("DELETE FROM files").Error; err != nil {
		t.Fatalf("clear files: %v", err)
	}
	if err := db.Exec("DELETE FROM nodes").Error; err != nil {
		t.Fatalf("clear nodes: %v", err)
	}
	cache.Init()
}

func seedOnlineNode(t *testing.T, uuid, ip string, port int) {
	t.Helper()

	if err := dbcore.DB().Create(&models.Node{
		UUID:     uuid,
		Token:    uuid + "-token",
		Hostname: uuid,
		IP:       ip,
		Port:     port,
		Status:   "online",
	}).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
}

func TestBrowseNodeFilesTreatsSlashAsManagedRoot(t *testing.T) {
	initNodeFilesTestDB(t)

	cache.FileTreeCache.Set("filetree:node-1", []agentapi.FileEntry{
		{Path: "/docs", Name: "docs", IsDir: true},
		{Path: "/docs/report.txt", Name: "report.txt", IsDir: false},
		{Path: "/notes.txt", Name: "notes.txt", IsDir: false},
	}, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "node-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/nodes/node-1/files?path=%2F", nil)

	BrowseNodeFiles(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entries []agentapi.FileEntry
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 root entries, got %d", len(entries))
	}
}

func TestNodeDownloadURLReturnsFilenameForFile(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	cache.FileTreeCache.Set("filetree:node-1", []agentapi.FileEntry{
		{Path: "/docs/report.txt", Name: "report.txt", IsDir: false},
	}, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "node-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/nodes/node-1/download?path=%2Fdocs%2Freport.txt", nil)

	NodeDownloadURL(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Filename != "report.txt" {
		t.Fatalf("expected report.txt, got %q", resp.Filename)
	}
	if !strings.Contains(resp.URL, "filename=report.txt") {
		t.Fatalf("expected filename in proxy url, got %q", resp.URL)
	}
}

func TestNodeDownloadURLReturnsArchiveForDirectory(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	cache.FileTreeCache.Set("filetree:node-1", []agentapi.FileEntry{
		{Path: "/docs", Name: "docs", IsDir: true},
	}, 0)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "uuid", Value: "node-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/nodes/node-1/download?path=%2Fdocs", nil)

	NodeDownloadURL(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Filename != "docs.zip" {
		t.Fatalf("expected docs.zip, got %q", resp.Filename)
	}
	if !strings.Contains(resp.URL, "archive=zip") {
		t.Fatalf("expected archive flag in proxy url, got %q", resp.URL)
	}
}

func TestInitUploadRejectsConflictBeforeCreatingRecords(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "existing-file",
		Name:   "report.txt",
		Path:   "/docs",
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:   "existing-file",
		NodeUUID: "node-1",
		Status:   "stored",
	}).Error; err != nil {
		t.Fatalf("create file replica: %v", err)
	}

	body := `{"node_uuid":"node-1","path":"/docs","name":"report.txt","size":12}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/files/upload", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	InitUpload(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}

	var fileCount int64
	if err := db.Model(&models.File{}).Count(&fileCount).Error; err != nil {
		t.Fatalf("count files: %v", err)
	}
	if fileCount != 1 {
		t.Fatalf("expected no new file rows, got %d", fileCount)
	}

	var replicaCount int64
	if err := db.Model(&models.FileReplica{}).Count(&replicaCount).Error; err != nil {
		t.Fatalf("count replicas: %v", err)
	}
	if replicaCount != 1 {
		t.Fatalf("expected no new replica rows, got %d", replicaCount)
	}
}

func TestInitUploadRejectsCachedNodeConflictBeforeCreatingRecords(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	cache.FileTreeCache.Set("filetree:node-1", []agentapi.FileEntry{
		{Path: "/docs/report.txt", Name: "report.txt", IsDir: false},
	}, 0)

	body := `{"node_uuid":"node-1","path":"/docs","name":"report.txt","size":12}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/files/upload", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	InitUpload(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}

	var fileCount int64
	if err := dbcore.DB().Model(&models.File{}).Count(&fileCount).Error; err != nil {
		t.Fatalf("count files: %v", err)
	}
	if fileCount != 0 {
		t.Fatalf("expected no file rows, got %d", fileCount)
	}

	var replicaCount int64
	if err := dbcore.DB().Model(&models.FileReplica{}).Count(&replicaCount).Error; err != nil {
		t.Fatalf("count replicas: %v", err)
	}
	if replicaCount != 0 {
		t.Fatalf("expected no replica rows, got %d", replicaCount)
	}
}

func TestInitUploadReturnsSingleProxyTarget(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	body := `{"node_uuid":"node-1","path":"/docs","name":"report.txt","size":12}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/files/upload", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	InitUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		FileID string `json:"file_id"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.FileID == "" {
		t.Fatalf("expected file_id in response")
	}
	if !strings.Contains(resp.URL, "/api/proxy/upload/") {
		t.Fatalf("expected proxy upload url, got %q", resp.URL)
	}

	var fileCount int64
	if err := dbcore.DB().Model(&models.File{}).Count(&fileCount).Error; err != nil {
		t.Fatalf("count files: %v", err)
	}
	if fileCount != 0 {
		t.Fatalf("expected init upload to avoid prewriting file rows, got %d", fileCount)
	}

	var replicaCount int64
	if err := dbcore.DB().Model(&models.FileReplica{}).Count(&replicaCount).Error; err != nil {
		t.Fatalf("count replicas: %v", err)
	}
	if replicaCount != 0 {
		t.Fatalf("expected init upload to avoid prewriting replica rows, got %d", replicaCount)
	}
}

func TestGetDownloadURLUsesManagedBrowsePath(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-1",
		Name:   "report.txt",
		Path:   "/docs",
		Size:   64,
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-1",
		NodeUUID:  "node-1",
		Status:    "stored",
		StorePath: "/srv/save_vps/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create replica: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "file-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/files/download/file-1", nil)

	GetDownloadURL(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Filename != "report.txt" {
		t.Fatalf("expected report.txt, got %q", resp.Filename)
	}
	if !strings.Contains(resp.URL, "/api/proxy/browse?") {
		t.Fatalf("expected managed browse proxy url, got %q", resp.URL)
	}
	if !strings.Contains(resp.URL, "path=%2Fdocs%2Freport.txt") {
		t.Fatalf("expected logical managed path in url, got %q", resp.URL)
	}
}

func TestProxyUploadForwardsPathMetadata(t *testing.T) {
	initNodeFilesTestDB(t)

	var capturedPath string
	var capturedName string
	var capturedOverwrite string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Query().Get("path")
		capturedName = r.URL.Query().Get("name")
		capturedOverwrite = r.URL.Query().Get("overwrite")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"stored"}`))
	}))
	defer server.Close()

	u, err := neturl.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	seedOnlineNode(t, "node-1", host, port)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPut, "/api/proxy/upload/file-1?node=node-1&path=%2Fdocs&name=report.txt&overwrite=true", bytes.NewBufferString("hello"))
	req.Header.Set("Content-Type", "text/plain")
	c.Params = gin.Params{{Key: "file_id", Value: "file-1"}}
	c.Request = req

	ProxyUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedPath != "/docs" {
		t.Fatalf("expected path /docs, got %q", capturedPath)
	}
	if capturedName != "report.txt" {
		t.Fatalf("expected report.txt, got %q", capturedName)
	}
	if capturedOverwrite != "true" {
		t.Fatalf("expected overwrite=true, got %q", capturedOverwrite)
	}
}

func TestProxyBrowseForwardsArchiveFlag(t *testing.T) {
	initNodeFilesTestDB(t)

	var capturedArchive string
	var capturedFilename string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedArchive = r.URL.Query().Get("archive")
		capturedFilename = r.URL.Query().Get("filename")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("zip"))
	}))
	defer server.Close()

	u, err := neturl.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	seedOnlineNode(t, "node-1", host, port)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/proxy/browse?node=node-1&path=%2Fdocs&filename=docs.zip&archive=zip", nil)

	ProxyBrowse(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedArchive != "zip" {
		t.Fatalf("expected archive=zip, got %q", capturedArchive)
	}
	if capturedFilename != "docs.zip" {
		t.Fatalf("expected filename docs.zip, got %q", capturedFilename)
	}
}

func TestProxyBrowseReturnsBadGatewayOnAgentTimeout(t *testing.T) {
	initNodeFilesTestDB(t)

	prevClient := agentProxyHTTPClient
	agentProxyHTTPClient = &http.Client{Timeout: 20 * time.Millisecond}
	defer func() {
		agentProxyHTTPClient = prevClient
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	u, err := neturl.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	seedOnlineNode(t, "node-timeout", host, port)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/proxy/browse?node=node-timeout&path=%2Fdocs%2Freport.txt", nil)

	ProxyBrowse(c)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestMoveFileMovesSingleStoredReplicaViaAgent(t *testing.T) {
	initNodeFilesTestDB(t)

	var capturedFrom string
	var capturedTo string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFrom = r.URL.Query().Get("from")
		capturedTo = r.URL.Query().Get("to")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"relative_path":"/archive/report-2026.txt","store_path":"/srv/data/files/archive/report-2026.txt"}`))
	}))
	defer server.Close()

	u, err := neturl.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	seedOnlineNode(t, "node-1", host, port)
	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-1",
		Name:   "report.txt",
		Path:   "/docs",
		Size:   32,
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-1",
		NodeUUID:  "node-1",
		Status:    "stored",
		StorePath: "/srv/data/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create replica: %v", err)
	}

	body := `{"new_path":"/archive","new_name":"report-2026.txt"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "file-1"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-1/move", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	MoveFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedFrom != "/docs/report.txt" {
		t.Fatalf("expected from path /docs/report.txt, got %q", capturedFrom)
	}
	if capturedTo != "/archive/report-2026.txt" {
		t.Fatalf("expected to path /archive/report-2026.txt, got %q", capturedTo)
	}

	var file models.File
	if err := db.Where("file_id = ?", "file-1").First(&file).Error; err != nil {
		t.Fatalf("load file: %v", err)
	}
	if file.Path != "/archive" || file.Name != "report-2026.txt" {
		t.Fatalf("expected moved file metadata, got path=%q name=%q", file.Path, file.Name)
	}

	var replica models.FileReplica
	if err := db.Where("file_id = ? AND node_uuid = ?", "file-1", "node-1").First(&replica).Error; err != nil {
		t.Fatalf("load replica: %v", err)
	}
	if replica.StorePath != "/srv/data/files/archive/report-2026.txt" {
		t.Fatalf("expected updated replica store_path, got %q", replica.StorePath)
	}
}

func TestMoveFileRejectsMultiReplicaFile(t *testing.T) {
	initNodeFilesTestDB(t)
	seedOnlineNode(t, "node-1", "127.0.0.1", 8801)
	seedOnlineNode(t, "node-2", "127.0.0.1", 8802)

	db := dbcore.DB()
	if err := db.Create(&models.File{
		FileID: "file-1",
		Name:   "report.txt",
		Path:   "/docs",
		Status: "ready",
	}).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-1",
		NodeUUID:  "node-1",
		Status:    "stored",
		StorePath: "/srv/data/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create replica node-1: %v", err)
	}
	if err := db.Create(&models.FileReplica{
		FileID:    "file-1",
		NodeUUID:  "node-2",
		Status:    "stored",
		StorePath: "/srv/data/files/docs/report.txt",
	}).Error; err != nil {
		t.Fatalf("create replica node-2: %v", err)
	}

	body := `{"new_path":"/archive","new_name":"report-2026.txt"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "file-1"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/files/file-1/move", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	MoveFile(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
}
