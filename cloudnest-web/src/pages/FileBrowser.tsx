import { useCallback, useEffect, useRef, useState } from "react";
import { Download, FileText, FolderSearch, Loader2, Search } from "lucide-react";
import { getDownloadURL, searchFiles, type StoredFile } from "../api/client";
import { triggerDownload } from "../utils/download";
import { useI18n } from "../i18n/useI18n";
import { EmptyState, PageHeader, SectionCard, SurfaceBox } from "../components/ui";

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
    timerRef.current = setTimeout(() => void doSearch(query), 300);
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
      <SurfaceBox className="flex items-center gap-3">
        <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent-muted text-accent">
          <Search className="h-5 w-5" />
        </span>
        <div className="min-w-0 flex-1">
          <label className="sr-only" htmlFor="file-search">{tx("搜索文件", "Search files")}</label>
          <input
            id="file-search"
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={tx("按文件名或路径搜索...", "Search files by name or path...")}
            className="w-full border-0 bg-transparent text-sm text-text-primary placeholder:text-text-muted outline-none"
            autoFocus
          />
        </div>
      </SurfaceBox>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-5 w-5 animate-spin text-accent" />
        </div>
      ) : null}

      {!loading && searched && results.length === 0 ? (
        <EmptyState
          icon={FolderSearch}
          title={tx("没有匹配文件", "No matching files")}
          description={tx(`未找到与 “${query}” 匹配的文件。`, `No files found for "${query}".`)}
        />
      ) : null}

      {!loading && results.length > 0 ? (
        <SectionCard title={tx("搜索结果", "Search Results")} description={tx("点击任意文件即可通过 Master 代理下载。", "Click any file to download it through the master proxy.")}> 
          <div className="overflow-hidden rounded-2xl border border-border">
            <div className="divide-y divide-border">
              {results.map((file) => (
                <button
                  key={file.file_id}
                  type="button"
                  onClick={() => void handleDownload(file.file_id)}
                  className="group flex w-full items-center gap-3 bg-card px-4 py-3 text-left transition-colors hover:bg-surface"
                >
                  <span className="flex h-10 w-10 items-center justify-center rounded-2xl bg-surface-subtle text-text-secondary">
                    <FileText className="h-4 w-4" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-text-primary">{file.name}</p>
                    <p className="truncate text-xs text-text-muted">{joinPath(file.path, file.name)}</p>
                  </div>
                  <span className="shrink-0 text-xs text-text-muted">{formatBytes(file.size)}</span>
                  <Download className="h-4 w-4 shrink-0 text-text-muted transition-colors group-hover:text-accent" />
                </button>
              ))}
            </div>
          </div>
        </SectionCard>
      ) : null}

      {!loading && !searched ? (
        <EmptyState
          icon={Search}
          title={tx("开始检索托管文件", "Search managed files")}
          description={tx("输入关键词后，系统会从托管文件元数据中返回匹配结果。", "Type a keyword and the system will return matches from managed file metadata.")}
        />
      ) : null}
    </div>
  );
}

export default function FileBrowser() {
  const { tx } = useI18n();

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={tx("托管文件", "Managed Files")}
        title={tx("文件检索", "File Search")}
        description={tx("这个页面保留全局托管文件搜索，不包含仅存在于节点扫描目录但尚未入库的文件。", "This page keeps global managed-file search and excludes files that only exist in scanned node directories but are not indexed yet.")}
      />
      <SearchTab />
    </div>
  );
}
