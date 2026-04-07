package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudnest/cloudnest-agent/internal/testutil"
)

func TestResolveManagedPathRejectsSymlinkDirectory(t *testing.T) {
	restore := withTempDataSaveDir(t)
	defer restore()

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	link := filepath.Join(FilesDir(), "shared")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	if _, _, err := ResolveManagedPath("/shared/report.txt"); err == nil {
		t.Fatal("expected symlink directory path to be rejected")
	}
}

func TestJoinManagedFilePathRejectsSymlinkTarget(t *testing.T) {
	restore := withTempDataSaveDir(t)
	defer restore()

	docsDir := filepath.Join(FilesDir(), "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	link := filepath.Join(docsDir, "report.txt")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	if _, _, err := JoinManagedFilePath("/docs", "report.txt"); err == nil {
		t.Fatal("expected symlink file target to be rejected")
	}
}

func TestRelativeManagedPathRejectsPathThroughSymlink(t *testing.T) {
	restore := withTempDataSaveDir(t)
	defer restore()

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	link := filepath.Join(FilesDir(), "shared")
	testutil.CreateSymlinkOrSkip(t, outside, link)

	if _, err := RelativeManagedPath(filepath.Join(FilesDir(), "shared", "report.txt")); err == nil {
		t.Fatal("expected symlink path to be rejected")
	}
}

func withTempDataSaveDir(t *testing.T) func() {
	t.Helper()
	root := t.TempDir()
	prev := os.Getenv("CLOUDNEST_DATA_SAVE_DIR")
	if err := os.Setenv("CLOUDNEST_DATA_SAVE_DIR", root); err != nil {
		t.Fatalf("set env: %v", err)
	}
	if err := EnsureDataDirs(); err != nil {
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
