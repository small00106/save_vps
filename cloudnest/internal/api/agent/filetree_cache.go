package agent

import "github.com/cloudnest/cloudnest/internal/cache"

func UpsertFileTreeEntry(nodeUUID string, entry FileEntry) {
	key := "filetree:" + nodeUUID
	raw, found := cache.FileTreeCache.Get(key)
	if !found {
		cache.FileTreeCache.Set(key, []FileEntry{entry}, 0)
		return
	}

	entries, ok := raw.([]FileEntry)
	if !ok {
		cache.FileTreeCache.Set(key, []FileEntry{entry}, 0)
		return
	}

	replaced := false
	for i := range entries {
		if entries[i].Path == entry.Path {
			entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, entry)
	}
	cache.FileTreeCache.Set(key, entries, 0)
}

func RemoveFileTreeEntry(nodeUUID, path string) {
	key := "filetree:" + nodeUUID
	raw, found := cache.FileTreeCache.Get(key)
	if !found {
		return
	}

	entries, ok := raw.([]FileEntry)
	if !ok {
		return
	}

	filtered := entries[:0]
	for _, entry := range entries {
		if entry.Path != path {
			filtered = append(filtered, entry)
		}
	}
	cache.FileTreeCache.Set(key, append([]FileEntry(nil), filtered...), 0)
}
