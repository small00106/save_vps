package storage

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DataSaveDir returns the base storage directory for agent data.
// Default: ~/data_save, can be overridden by CLOUDNEST_DATA_SAVE_DIR.
func DataSaveDir() string {
	if custom := strings.TrimSpace(os.Getenv("CLOUDNEST_DATA_SAVE_DIR")); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "./data_save"
	}
	return filepath.Join(home, "data_save")
}

// FilesDir returns the root directory for uploaded file blobs.
func FilesDir() string {
	return filepath.Join(DataSaveDir(), "files")
}

// EnsureDataDirs creates required local storage directories.
func EnsureDataDirs() error {
	return os.MkdirAll(FilesDir(), 0755)
}

// FilePath returns the full storage path for a given file ID.
func FilePath(fileID string) (string, error) {
	if len(fileID) < 2 {
		return "", fmt.Errorf("invalid file id")
	}
	return filepath.Join(FilesDir(), fileID[:2], fileID), nil
}

// EnsureShardDir creates and returns the shard directory for a file ID.
func EnsureShardDir(fileID string) (string, error) {
	if len(fileID) < 2 {
		return "", fmt.Errorf("invalid file id")
	}
	dir := filepath.Join(FilesDir(), fileID[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// NormalizeManagedDirPath normalizes a logical managed directory path to a slash path.
func NormalizeManagedDirPath(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" || trimmed == "." {
		return "/"
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

// ResolveManagedPath resolves a logical managed path under FilesDir.
func ResolveManagedPath(relPath string) (string, string, error) {
	root := filepath.Clean(FilesDir())
	normalized := NormalizeManagedDirPath(relPath)
	if normalized == "/" {
		return root, normalized, nil
	}

	candidate := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(normalized, "/")))
	relToRoot, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", "", err
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path traversal not allowed")
	}
	return candidate, normalized, nil
}

// JoinManagedFilePath resolves a logical managed directory and file name to an absolute path.
func JoinManagedFilePath(dirPath, name string) (string, string, error) {
	baseName := strings.TrimSpace(name)
	if baseName == "" {
		return "", "", fmt.Errorf("file name required")
	}
	baseName = filepath.Base(baseName)
	if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
		return "", "", fmt.Errorf("invalid file name")
	}

	dirAbs, dirRel, err := ResolveManagedPath(dirPath)
	if err != nil {
		return "", "", err
	}

	fileAbs := filepath.Join(dirAbs, baseName)
	relative := path.Join(dirRel, baseName)
	if !strings.HasPrefix(relative, "/") {
		relative = "/" + relative
	}
	return fileAbs, relative, nil
}

// RelativeManagedPath returns the logical managed path for an absolute path inside FilesDir.
func RelativeManagedPath(absPath string) (string, error) {
	root := filepath.Clean(FilesDir())
	cleanPath := filepath.Clean(absPath)
	rel, err := filepath.Rel(root, cleanPath)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "/", nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path is outside managed root")
	}
	return "/" + filepath.ToSlash(rel), nil
}
