import { useState, useEffect, useRef, useCallback } from "react";
import {
  Search, FileText, Loader2, FolderSearch, Download, Upload, Plus, X,
  Folder, ChevronRight, ArrowLeft, Trash2, CheckCircle2, AlertCircle,
  HardDrive,
} from "lucide-react";
import {
  searchFiles, getNodeDownloadURL, getNodes, listFiles, initUpload,
  getDownloadURL, createDir, deleteFile,
  type SearchResult, type StoredFile, type Node,
} from "../api/client";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

// ========================
// Upload target progress tracking
// ========================

interface UploadTarget {
  nodeUUID: string;
  hostname: string;
  progress: number; // 0-100
  status: "pending" | "uploading" | "done" | "error";
  error?: string;
}

function uploadFileToAgent(
  url: string,
  file: File,
  onProgress: (pct: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url, true);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`HTTP ${xhr.status}`)));
    xhr.onerror = () => reject(new Error("Network error"));
    xhr.send(file);
  });
}

// ========================
// Search sub-component (extracted from old FileBrowser)
// ========================

function SearchTab() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) { setResults([]); setSearched(false); return; }
    setLoading(true);
    setSearched(true);
    try {
      const data = await searchFiles(q.trim());
      setResults(Array.isArray(data) ? data : []);
    } catch { setResults([]); } finally { setLoading(false); }
  }, []);

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => doSearch(query), 300);
    return () => { if (timerRef.current) clearTimeout(timerRef.current); };
  }, [query, doSearch]);

  const handleDownload = async (nodeUuid: string, path: string) => {
    try {
      const res = await getNodeDownloadURL(nodeUuid, path);
      window.open(res.url, "_blank");
    } catch { /* ignore */ }
  };

  const grouped = results.reduce<Record<string, SearchResult[]>>((acc, r) => {
    (acc[r.node_uuid] ||= []).push(r);
    return acc;
  }, {});

  return (
    <div className="space-y-4">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#71717a]" />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search files across all nodes..."
          className="w-full h-10 pl-10 pr-4 rounded-lg bg-[#18181b] border border-[#27272a] text-[#fafafa] text-sm placeholder-[#71717a] focus:outline-none focus:border-[#3b82f6] transition-colors"
          autoFocus
        />
      </div>

      {loading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-5 h-5 text-[#3b82f6] animate-spin" />
        </div>
      )}

      {!loading && searched && results.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-[#71717a]">
          <FolderSearch className="w-10 h-10 mb-3" />
          <p className="text-sm">No files found for &quot;{query}&quot;</p>
        </div>
      )}

      {!loading && Object.keys(grouped).length > 0 && (
        <div className="space-y-4">
          {Object.entries(grouped).map(([nodeUuid, items]) => (
            <div key={nodeUuid} className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden">
              <div className="px-4 py-2.5 border-b border-[#27272a] bg-[#09090b]">
                <span className="text-xs font-mono text-[#a1a1aa]">Node: {nodeUuid.slice(0, 12)}...</span>
              </div>
              <div className="divide-y divide-[#27272a]">
                {items.map((r, i) => (
                  <button
                    key={i}
                    onClick={() => handleDownload(nodeUuid, r.entry.path)}
                    className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors text-left group"
                  >
                    <FileText className="w-4 h-4 text-[#71717a] shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-[#fafafa] truncate">{r.entry.name}</p>
                      <p className="text-xs text-[#71717a] truncate">{r.entry.path}</p>
                    </div>
                    <span className="text-xs text-[#71717a] shrink-0">{formatBytes(r.entry.size)}</span>
                    <Download className="w-3 h-3 text-[#71717a] opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && !searched && (
        <div className="flex flex-col items-center justify-center py-16 text-[#71717a]">
          <Search className="w-10 h-10 mb-3" />
          <p className="text-sm">Type to search files across all nodes</p>
        </div>
      )}
    </div>
  );
}

// ========================
// Files tab (virtual file manager + upload)
// ========================

function FilesTab() {
  const [currentPath, setCurrentPath] = useState("/");
  const [files, setFiles] = useState<StoredFile[]>([]);
  const [loading, setLoading] = useState(true);

  // Upload state
  const [showUpload, setShowUpload] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadTargets, setUploadTargets] = useState<UploadTarget[]>([]);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // New folder state
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [folderName, setFolderName] = useState("");
  const [creatingFolder, setCreatingFolder] = useState(false);

  const loadFiles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listFiles(currentPath);
      setFiles(Array.isArray(data) ? data : []);
    } catch { setFiles([]); } finally { setLoading(false); }
  }, [currentPath]);

  useEffect(() => { loadFiles(); }, [loadFiles]);

  // Load nodes when upload panel opens
  useEffect(() => {
    if (!showUpload) return;
    getNodes().then((data) => {
      const online = (Array.isArray(data) ? data : []).filter((n) => n.status === "online");
      setNodes(online);
      if (online.length > 0 && selectedNodes.length === 0) {
        setSelectedNodes([online[0].uuid]);
      }
    }).catch(() => setNodes([]));
  }, [showUpload]);

  // Breadcrumbs
  const breadcrumbs = (() => {
    const parts = currentPath.split("/").filter(Boolean);
    const crumbs = [{ label: "Root", path: "/" }];
    let acc = "";
    for (const p of parts) { acc += "/" + p; crumbs.push({ label: p, path: acc }); }
    return crumbs;
  })();

  const toggleNode = (uuid: string) => {
    setSelectedNodes((prev) =>
      prev.includes(uuid) ? prev.filter((u) => u !== uuid) : [...prev, uuid],
    );
  };

  const handleUpload = async () => {
    if (!selectedFile || selectedNodes.length === 0) return;
    setUploading(true);

    const targets: UploadTarget[] = selectedNodes.map((uuid) => ({
      nodeUUID: uuid,
      hostname: nodes.find((n) => n.uuid === uuid)?.hostname || uuid.slice(0, 8),
      progress: 0,
      status: "pending",
    }));
    setUploadTargets(targets);

    try {
      const res = await initUpload({
        name: selectedFile.name,
        size: selectedFile.size,
        path: currentPath,
        node_uuids: selectedNodes,
      });

      // Upload to each target in parallel
      await Promise.all(
        res.targets.map(async (target) => {
          const idx = targets.findIndex((t) => t.nodeUUID === target.node_uuid);
          if (idx === -1) return;

          setUploadTargets((prev) => {
            const next = [...prev];
            next[idx] = { ...next[idx], status: "uploading" };
            return next;
          });

          try {
            await uploadFileToAgent(target.url, selectedFile!, (pct) => {
              setUploadTargets((prev) => {
                const next = [...prev];
                next[idx] = { ...next[idx], progress: pct };
                return next;
              });
            });
            setUploadTargets((prev) => {
              const next = [...prev];
              next[idx] = { ...next[idx], status: "done", progress: 100 };
              return next;
            });
          } catch (err) {
            setUploadTargets((prev) => {
              const next = [...prev];
              next[idx] = { ...next[idx], status: "error", error: String(err) };
              return next;
            });
          }
        }),
      );

      // Refresh file list after upload
      setTimeout(() => loadFiles(), 500);
    } catch {
      setUploadTargets((prev) => prev.map((t) => ({ ...t, status: "error", error: "Failed to initialize upload" })));
    }

    setUploading(false);
  };

  const resetUpload = () => {
    setShowUpload(false);
    setSelectedFile(null);
    setSelectedNodes([]);
    setUploadTargets([]);
  };

  const handleCreateFolder = async () => {
    if (!folderName.trim()) return;
    setCreatingFolder(true);
    try {
      await createDir(currentPath, folderName.trim());
      setShowNewFolder(false);
      setFolderName("");
      loadFiles();
    } catch { /* ignore */ }
    setCreatingFolder(false);
  };

  const handleDelete = async (fileId: string, fileName: string) => {
    if (!confirm(`Delete "${fileName}"?`)) return;
    try {
      await deleteFile(fileId);
      setFiles((prev) => prev.filter((f) => f.file_id !== fileId));
    } catch { /* ignore */ }
  };

  const handleDownload = async (fileId: string) => {
    try {
      const res = await getDownloadURL(fileId);
      window.open(res.url, "_blank");
    } catch { /* ignore */ }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files[0];
    if (file) setSelectedFile(file);
  };

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center gap-2 flex-wrap">
        <button
          onClick={() => { setShowUpload(!showUpload); setShowNewFolder(false); }}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors"
        >
          {showUpload ? <X className="w-4 h-4" /> : <Upload className="w-4 h-4" />}
          {showUpload ? "Cancel" : "Upload"}
        </button>
        <button
          onClick={() => { setShowNewFolder(!showNewFolder); setShowUpload(false); }}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#18181b] border border-[#27272a] hover:bg-[#232329] text-[#fafafa] text-sm font-medium transition-colors"
        >
          {showNewFolder ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          {showNewFolder ? "Cancel" : "New Folder"}
        </button>
      </div>

      {/* Upload panel */}
      {showUpload && (
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 space-y-4">
          {/* File selection (drag & drop) */}
          <div>
            <label className="block text-xs text-[#a1a1aa] mb-2">File</label>
            <div
              onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
              onDragLeave={() => setDragOver(false)}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className={`flex flex-col items-center justify-center h-28 rounded-lg border-2 border-dashed cursor-pointer transition-colors ${
                dragOver
                  ? "border-[#3b82f6] bg-[#3b82f6]/5"
                  : selectedFile
                    ? "border-[#22c55e]/50 bg-[#22c55e]/5"
                    : "border-[#27272a] hover:border-[#3b82f6]/50"
              }`}
            >
              {selectedFile ? (
                <div className="text-center">
                  <FileText className="w-6 h-6 text-[#22c55e] mx-auto mb-1" />
                  <p className="text-sm text-[#fafafa]">{selectedFile.name}</p>
                  <p className="text-xs text-[#71717a]">{formatBytes(selectedFile.size)}</p>
                </div>
              ) : (
                <div className="text-center">
                  <Upload className="w-6 h-6 text-[#71717a] mx-auto mb-1" />
                  <p className="text-sm text-[#71717a]">Drop file here or click to browse</p>
                </div>
              )}
            </div>
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              onChange={(e) => { if (e.target.files?.[0]) setSelectedFile(e.target.files[0]); }}
            />
          </div>

          {/* Target node selection */}
          <div>
            <label className="block text-xs text-[#a1a1aa] mb-2">Target Nodes</label>
            {nodes.length === 0 ? (
              <p className="text-sm text-[#71717a]">No online nodes available</p>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
                {nodes.map((node) => {
                  const checked = selectedNodes.includes(node.uuid);
                  return (
                    <label
                      key={node.uuid}
                      className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors ${
                        checked
                          ? "border-[#3b82f6] bg-[#3b82f6]/5"
                          : "border-[#27272a] hover:border-[#3b82f6]/40"
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleNode(node.uuid)}
                        className="sr-only"
                      />
                      <div className={`w-4 h-4 rounded border flex items-center justify-center shrink-0 ${
                        checked ? "bg-[#3b82f6] border-[#3b82f6]" : "border-[#71717a]"
                      }`}>
                        {checked && <CheckCircle2 className="w-3 h-3 text-white" />}
                      </div>
                      <HardDrive className="w-4 h-4 text-[#71717a] shrink-0" />
                      <div className="min-w-0">
                        <p className="text-sm text-[#fafafa] truncate">{node.hostname}</p>
                        <p className="text-[10px] text-[#71717a]">{node.ip}</p>
                      </div>
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          {/* Upload progress */}
          {uploadTargets.length > 0 && (
            <div className="space-y-2">
              <label className="block text-xs text-[#a1a1aa]">Progress</label>
              {uploadTargets.map((t) => (
                <div key={t.nodeUUID} className="flex items-center gap-3">
                  <span className="text-xs text-[#a1a1aa] w-20 truncate shrink-0">{t.hostname}</span>
                  <div className="flex-1 h-2 rounded-full bg-[#27272a] overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all duration-300 ${
                        t.status === "error" ? "bg-[#ef4444]" : t.status === "done" ? "bg-[#22c55e]" : "bg-[#3b82f6]"
                      }`}
                      style={{ width: `${t.progress}%` }}
                    />
                  </div>
                  <span className="text-xs w-10 text-right shrink-0">
                    {t.status === "done" ? (
                      <CheckCircle2 className="w-4 h-4 text-[#22c55e] inline" />
                    ) : t.status === "error" ? (
                      <AlertCircle className="w-4 h-4 text-[#ef4444] inline" />
                    ) : (
                      <span className="text-[#a1a1aa]">{t.progress}%</span>
                    )}
                  </span>
                </div>
              ))}
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center gap-3">
            {uploadTargets.length > 0 && uploadTargets.every((t) => t.status === "done" || t.status === "error") ? (
              <button
                onClick={resetUpload}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#27272a] hover:bg-[#3f3f46] text-[#fafafa] text-sm font-medium transition-colors"
              >
                Done
              </button>
            ) : (
              <button
                onClick={handleUpload}
                disabled={uploading || !selectedFile || selectedNodes.length === 0}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors disabled:opacity-50"
              >
                {uploading && <Loader2 className="w-4 h-4 animate-spin" />}
                Upload to {selectedNodes.length} node{selectedNodes.length !== 1 ? "s" : ""}
              </button>
            )}
          </div>
        </div>
      )}

      {/* New folder panel */}
      {showNewFolder && (
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl p-5 flex items-end gap-3">
          <div className="flex-1">
            <label className="block text-xs text-[#a1a1aa] mb-1">Folder Name</label>
            <input
              value={folderName}
              onChange={(e) => setFolderName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleCreateFolder()}
              className="w-full h-9 px-3 rounded-lg bg-[#09090b] border border-[#27272a] text-white text-sm focus:outline-none focus:border-[#3b82f6] transition-colors"
              placeholder="my-folder"
              autoFocus
            />
          </div>
          <button
            onClick={handleCreateFolder}
            disabled={creatingFolder || !folderName.trim()}
            className="flex items-center gap-2 h-9 px-4 rounded-lg bg-[#3b82f6] hover:bg-blue-600 text-white text-sm font-medium transition-colors disabled:opacity-50"
          >
            {creatingFolder && <Loader2 className="w-4 h-4 animate-spin" />}
            Create
          </button>
        </div>
      )}

      {/* File browser */}
      <div className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden">
        {/* Breadcrumbs */}
        <div className="flex items-center gap-1 px-4 py-3 border-b border-[#27272a] text-sm overflow-x-auto">
          {breadcrumbs.map((crumb, i) => (
            <span key={crumb.path} className="flex items-center gap-1 shrink-0">
              {i > 0 && <ChevronRight className="w-3 h-3 text-[#71717a]" />}
              <button
                onClick={() => setCurrentPath(crumb.path)}
                className={`hover:text-[#3b82f6] transition-colors ${
                  i === breadcrumbs.length - 1 ? "text-[#fafafa]" : "text-[#71717a]"
                }`}
              >
                {crumb.label}
              </button>
            </span>
          ))}
        </div>

        {loading ? (
          <div className="flex items-center justify-center h-48">
            <Loader2 className="w-5 h-5 text-[#3b82f6] animate-spin" />
          </div>
        ) : files.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-48 text-[#71717a]">
            <Folder className="w-8 h-8 mb-2" />
            <p className="text-sm">Empty directory</p>
            <p className="text-xs mt-1">Upload a file or create a folder to get started</p>
          </div>
        ) : (
          <div className="divide-y divide-[#27272a]">
            {currentPath !== "/" && (
              <button
                onClick={() => {
                  const parent = currentPath.split("/").slice(0, -1).join("/") || "/";
                  setCurrentPath(parent);
                }}
                className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors text-left"
              >
                <ArrowLeft className="w-4 h-4 text-[#71717a]" />
                <span className="text-sm text-[#a1a1aa]">..</span>
              </button>
            )}
            {[...files]
              .sort((a, b) => (a.is_dir === b.is_dir ? a.name.localeCompare(b.name) : a.is_dir ? -1 : 1))
              .map((file) => (
                <div
                  key={file.file_id}
                  className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors group"
                >
                  <button
                    onClick={() => file.is_dir ? setCurrentPath(currentPath === "/" ? "/" + file.name : currentPath + "/" + file.name) : handleDownload(file.file_id)}
                    className="flex items-center gap-3 flex-1 min-w-0 text-left"
                  >
                    {file.is_dir ? (
                      <Folder className="w-4 h-4 text-[#3b82f6] shrink-0" />
                    ) : (
                      <FileText className="w-4 h-4 text-[#71717a] shrink-0" />
                    )}
                    <span className="text-sm text-[#fafafa] truncate">{file.name}</span>
                    {!file.is_dir && (
                      <span className="text-xs text-[#71717a] shrink-0 ml-auto">{formatBytes(file.size)}</span>
                    )}
                  </button>
                  <div className="flex items-center gap-1 shrink-0">
                    {!file.is_dir && file.status === "ready" && (
                      <button
                        onClick={() => handleDownload(file.file_id)}
                        className="p-1 rounded text-[#71717a] hover:text-[#3b82f6] opacity-0 group-hover:opacity-100 transition-all"
                      >
                        <Download className="w-3.5 h-3.5" />
                      </button>
                    )}
                    {!file.is_dir && file.status === "uploading" && (
                      <span className="text-[10px] text-[#f59e0b] bg-[#f59e0b]/10 px-1.5 py-0.5 rounded">uploading</span>
                    )}
                    <button
                      onClick={() => handleDelete(file.file_id, file.name)}
                      className="p-1 rounded text-[#71717a] hover:text-[#ef4444] opacity-0 group-hover:opacity-100 transition-all"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ========================
// Main page with tabs
// ========================

export default function FileBrowser() {
  const [tab, setTab] = useState<"files" | "search">("files");

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-[#fafafa]">Files</h1>
        <div className="flex gap-1 bg-[#18181b] border border-[#27272a] rounded-lg p-1">
          {(["files", "search"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                tab === t ? "bg-[#27272a] text-[#fafafa]" : "text-[#71717a] hover:text-[#a1a1aa]"
              }`}
            >
              {t === "files" ? "Files" : "Search"}
            </button>
          ))}
        </div>
      </div>

      {tab === "files" ? <FilesTab /> : <SearchTab />}
    </div>
  );
}
