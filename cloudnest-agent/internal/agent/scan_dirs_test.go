package agent

import (
	"testing"

	"github.com/cloudnest/cloudnest-agent/internal/storage"
)

func TestManagedScanDirsAlwaysUsesFilesDir(t *testing.T) {
	restoreEnv := withRegisterEnv(t)
	defer restoreEnv()

	cfg := &Config{
		ScanDirs: []string{"/tmp/legacy-data-save"},
	}

	got := managedScanDirs(cfg)
	if len(got) != 1 || got[0] != storage.FilesDir() {
		t.Fatalf("expected managed scan dir %q, got %#v", storage.FilesDir(), got)
	}
}
