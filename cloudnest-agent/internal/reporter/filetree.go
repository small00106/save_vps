package reporter

import (
	"os"
	"path/filepath"
	"time"
)

type FileEntry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}

// FileTreeDiff represents incremental changes.
type FileTreeDiff struct {
	Added   []FileEntry `json:"added"`
	Removed []string    `json:"removed"`
}

// ScanDirectories scans all configured directories and returns a flat file list.
func ScanDirectories(dirs []string) []FileEntry {
	var entries []FileEntry

	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			entries = append(entries, FileEntry{
				Path:    path,
				Name:    info.Name(),
				Size:    info.Size(),
				IsDir:   info.IsDir(),
				ModTime: info.ModTime(),
			})
			return nil
		})
	}

	return entries
}

// DiffFileTrees computes incremental changes between previous and current scans.
func DiffFileTrees(prev, curr []FileEntry) *FileTreeDiff {
	prevMap := make(map[string]FileEntry, len(prev))
	for _, e := range prev {
		prevMap[e.Path] = e
	}

	currMap := make(map[string]FileEntry, len(curr))
	for _, e := range curr {
		currMap[e.Path] = e
	}

	diff := &FileTreeDiff{}

	// Find added or modified entries
	for path, ce := range currMap {
		pe, exists := prevMap[path]
		if !exists || ce.Size != pe.Size || !ce.ModTime.Equal(pe.ModTime) {
			diff.Added = append(diff.Added, ce)
		}
	}

	// Find removed entries
	for path := range prevMap {
		if _, exists := currMap[path]; !exists {
			diff.Removed = append(diff.Removed, path)
		}
	}

	return diff
}
