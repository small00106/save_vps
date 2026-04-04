package reporter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectoriesReportsRelativePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	entries := ScanDirectories([]string{root})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	got := map[string]FileEntry{}
	for _, entry := range entries {
		got[entry.Path] = entry
	}

	if _, ok := got["/nested"]; !ok {
		t.Fatalf("expected directory /nested in %#v", got)
	}
	fileEntry, ok := got["/nested/hello.txt"]
	if !ok {
		t.Fatalf("expected file /nested/hello.txt in %#v", got)
	}
	if fileEntry.Name != "hello.txt" {
		t.Fatalf("expected hello.txt, got %q", fileEntry.Name)
	}
}
