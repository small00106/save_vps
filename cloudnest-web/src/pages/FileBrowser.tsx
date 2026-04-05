import { useCallback, useEffect, useRef, useState } from "react";
import { Download, FileText, FolderSearch, Loader2, Search } from "lucide-react";
import {
  getDownloadURL,
  searchFiles,
  type StoredFile,
} from "../api/client";
import { triggerDownload } from "../utils/download";
import { useI18n } from "../i18n/useI18n";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

function joinPath(path: string, name: string): string {
  const normalized = path === "/" ? "" : path.replace(/\/+$/, "");
  return `${normalized}/${name}`;
}

function SearchTab() {
  const { tx } = useI18n();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<StoredFile[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const doSearch = useCallback(async (value: string) => {
    const trimmed = value.trim();
    if (!trimmed) {
      setResults([]);
      setSearched(false);
      return;
    }
    setLoading(true);
    setSearched(true);
    try {
      const data = await searchFiles(trimmed);
      setResults(Array.isArray(data) ? data : []);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => doSearch(query), 300);
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [query, doSearch]);

  const handleDownload = async (fileId: string) => {
    const res = await getDownloadURL(fileId);
    triggerDownload(res.url, res.filename);
  };

  return (
    <div className="space-y-4">
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted" />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={tx("按文件名或路径搜索...", "Search files by name or path...")}
          className="h-10 w-full rounded-lg border border-border bg-card pl-10 pr-4 text-sm text-text-primary placeholder-text-muted transition-colors focus:border-accent focus:outline-none"
          autoFocus
        />
      </div>

      {loading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-5 w-5 animate-spin text-accent" />
        </div>
      )}

      {!loading && searched && results.length === 0 && (
        <div className="flex flex-col items-center justify-center py-16 text-text-muted">
          <FolderSearch className="w-10 h-10 mb-3" />
          <p className="text-sm">
            {tx(`未找到与 “${query}” 匹配的文件`, `No files found for "${query}"`)}
          </p>
        </div>
      )}

      {!loading && results.length > 0 && (
        <div className="overflow-hidden rounded-xl border border-border bg-card">
          <div className="divide-y divide-border">
            {results.map((file) => (
              <button
                key={file.file_id}
                onClick={() => void handleDownload(file.file_id)}
                className="group flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors hover:bg-border/50"
              >
                <FileText className="h-4 w-4 shrink-0 text-text-muted" />
                <div className="flex-1 min-w-0">
                  <p className="truncate text-sm text-text-primary">{file.name}</p>
                  <p className="truncate text-xs text-text-muted">{joinPath(file.path, file.name)}</p>
                </div>
                <span className="shrink-0 text-xs text-text-muted">{formatBytes(file.size)}</span>
                <Download className="h-3 w-3 shrink-0 text-text-muted opacity-0 transition-opacity group-hover:opacity-100" />
              </button>
            ))}
          </div>
        </div>
      )}

      {!loading && !searched && (
        <div className="flex flex-col items-center justify-center py-16 text-text-muted">
          <Search className="w-10 h-10 mb-3" />
          <p className="text-sm">
            {tx("输入关键词开始搜索文件", "Type to search files by name or path")}
          </p>
        </div>
      )}
    </div>
  );
}

export default function FileBrowser() {
  const { tx } = useI18n();

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-text-primary">{tx("文件", "Files")}</h1>
        <span className="rounded-lg border border-border bg-card px-3 py-1 text-sm text-text-secondary">
          {tx("仅搜索", "Search only")}
        </span>
      </div>

      <SearchTab />
    </div>
  );
}
