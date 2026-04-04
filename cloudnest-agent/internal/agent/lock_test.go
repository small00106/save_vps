package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRunLockRejectsActiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.lock")
	writeLockFile(t, path, lockMetadata{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	})

	lock, err := acquireRunLock(path)
	if lock != nil {
		t.Fatalf("expected no lock, got %#v", lock)
	}
	if !errors.Is(err, ErrAgentAlreadyRunning) {
		t.Fatalf("expected ErrAgentAlreadyRunning, got %v", err)
	}
}

func TestAcquireRunLockReclaimsStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.lock")
	writeLockFile(t, path, lockMetadata{
		PID:       424242,
		CreatedAt: time.Now().Add(-time.Hour).UTC(),
	})

	restore := swapProcessAliveFunc(func(pid int) bool {
		return false
	})
	defer restore()

	lock, err := acquireRunLock(path)
	if err != nil {
		t.Fatalf("expected stale lock to be reclaimed, got %v", err)
	}
	t.Cleanup(func() {
		if lock != nil {
			_ = lock.Release()
		}
	})

	meta := readLockFile(t, path)
	if meta.PID != os.Getpid() {
		t.Fatalf("expected lock PID %d, got %d", os.Getpid(), meta.PID)
	}
}

func TestAcquireRunLockReclaimsExpiredCorruptLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.lock")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupt lock: %v", err)
	}

	old := time.Now().Add(-2 * staleLockCorruptionGracePeriod)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes lock: %v", err)
	}

	lock, err := acquireRunLock(path)
	if err != nil {
		t.Fatalf("expected corrupt stale lock to be reclaimed, got %v", err)
	}
	t.Cleanup(func() {
		if lock != nil {
			_ = lock.Release()
		}
	})
}

func writeLockFile(t *testing.T, path string, meta lockMetadata) {
	t.Helper()
	data, err := marshalLockMetadata(meta)
	if err != nil {
		t.Fatalf("marshal lock metadata: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}

func readLockFile(t *testing.T, path string) lockMetadata {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	meta, err := unmarshalLockMetadata(data)
	if err != nil {
		t.Fatalf("unmarshal lock metadata: %v", err)
	}
	return meta
}

