package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var ErrAgentAlreadyRunning = errors.New("agent is already running for this user")

const staleLockCorruptionGracePeriod = 10 * time.Second

var processAliveFunc = isProcessAlive

type runLock struct {
	path string
}

type lockMetadata struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

func defaultRunLockPath() string {
	home := resolveHomeDir()
	return filepath.Join(home, ".cloudnest", "agent.run.lock")
}

func acquireDefaultRunLock() (*runLock, error) {
	return acquireRunLock(defaultRunLockPath())
}

func acquireRunLock(path string) (*runLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	for attempts := 0; attempts < 3; attempts++ {
		lock, err := createRunLock(path)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}

		stale, owner, staleErr := isStaleLock(path)
		if staleErr != nil {
			return nil, staleErr
		}
		if !stale {
			if owner.PID > 0 {
				return nil, fmt.Errorf("%w (pid=%d)", ErrAgentAlreadyRunning, owner.PID)
			}
			return nil, ErrAgentAlreadyRunning
		}

		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("remove stale lock: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to acquire agent lock at %s", path)
}

func createRunLock(path string) (*runLock, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}

	data, err := marshalLockMetadata(lockMetadata{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("marshal lock metadata: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock metadata: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close lock file: %w", err)
	}

	return &runLock{path: path}, nil
}

func isStaleLock(path string) (bool, lockMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, lockMetadata{}, nil
		}
		return false, lockMetadata{}, fmt.Errorf("read lock metadata: %w", err)
	}

	meta, err := unmarshalLockMetadata(data)
	if err != nil {
		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				return true, lockMetadata{}, nil
			}
			return false, lockMetadata{}, fmt.Errorf("stat corrupt lock: %w", statErr)
		}
		if time.Since(info.ModTime()) >= staleLockCorruptionGracePeriod {
			return true, lockMetadata{}, nil
		}
		return false, lockMetadata{}, fmt.Errorf("%w (lock metadata unreadable)", ErrAgentAlreadyRunning)
	}

	if meta.PID <= 0 {
		return true, meta, nil
	}

	return !processAliveFunc(meta.PID), meta, nil
}

func marshalLockMetadata(meta lockMetadata) ([]byte, error) {
	return json.Marshal(meta)
}

func unmarshalLockMetadata(data []byte) (lockMetadata, error) {
	var meta lockMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return lockMetadata{}, err
	}
	return meta, nil
}

func (l *runLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
