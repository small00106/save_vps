package testutil

import (
	"os"
	"testing"
)

// CreateSymlinkOrSkip creates a symbolic link when the environment supports it.
// Some Windows setups require elevated privileges or developer mode.
func CreateSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}
}
