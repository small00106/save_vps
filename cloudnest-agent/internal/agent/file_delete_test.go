package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudnest/cloudnest-agent/internal/storage"
	"github.com/cloudnest/cloudnest-agent/internal/testutil"
	"github.com/cloudnest/cloudnest-agent/internal/ws"
)

func TestDeleteFileRemovesManagedStorePath(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	target := filepath.Join(storage.FilesDir(), "docs", "report.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("mkdir target dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("payload"), 0644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	params, err := json.Marshal(map[string]string{
		"file_id":    "file-123",
		"store_path": target,
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	deleteFile(&ws.Client{}, params)

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
}

func TestDeleteFileRejectsPathThroughSymlinkDirectory(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	outsideDir := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("mkdir outside dir: %v", err)
	}

	outsideFile := filepath.Join(outsideDir, "report.txt")
	if err := os.WriteFile(outsideFile, []byte("payload"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	linkDir := filepath.Join(storage.FilesDir(), "shared")
	testutil.CreateSymlinkOrSkip(t, outsideDir, linkDir)

	params, err := json.Marshal(map[string]string{
		"file_id":    "file-123",
		"store_path": filepath.Join(linkDir, "report.txt"),
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	deleteFile(&ws.Client{}, params)

	if _, err := os.Stat(outsideFile); err != nil {
		t.Fatalf("expected outside file to remain, stat err=%v", err)
	}
}
