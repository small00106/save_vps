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
	if len(got) != 2 || got[0] != storage.FilesDir() || got[1] != "/tmp/legacy-data-save" {
		t.Fatalf("expected [%q, %q], got %#v", storage.FilesDir(), "/tmp/legacy-data-save", got)
	}
}
