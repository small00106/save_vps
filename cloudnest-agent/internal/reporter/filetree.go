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

// ScanDirectories scans all configured directories and returns a flat file list.
func ScanDirectories(dirs []string) []FileEntry {
	var entries []FileEntry

	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip errors
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
