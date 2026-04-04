import { useCallback, useEffect, useRef, useState } from "react";
import { Download, FileText, FolderSearch, Loader2, Search } from "lucide-react";
import {
  getDownloadURL,
  searchFiles,
  type StoredFile,
} from "../api/client";
import { triggerDownload } from "../utils/download";

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
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#71717a]" />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search files by name or path..."
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

      {!loading && results.length > 0 && (
        <div className="bg-[#18181b] border border-[#27272a] rounded-xl overflow-hidden">
          <div className="divide-y divide-[#27272a]">
            {results.map((file) => (
              <button
                key={file.file_id}
                onClick={() => void handleDownload(file.file_id)}
                className="flex items-center gap-3 w-full px-4 py-2.5 hover:bg-[#232329] transition-colors text-left group"
              >
                <FileText className="w-4 h-4 text-[#71717a] shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-[#fafafa] truncate">{file.name}</p>
                  <p className="text-xs text-[#71717a] truncate">{joinPath(file.path, file.name)}</p>
                </div>
                <span className="text-xs text-[#71717a] shrink-0">{formatBytes(file.size)}</span>
                <Download className="w-3 h-3 text-[#71717a] opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
              </button>
            ))}
          </div>
        </div>
      )}

      {!loading && !searched && (
        <div className="flex flex-col items-center justify-center py-16 text-[#71717a]">
          <Search className="w-10 h-10 mb-3" />
          <p className="text-sm">Type to search files by name or path</p>
        </div>
      )}
    </div>
  );
}

export default function FileBrowser() {
  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-[#fafafa]">Files</h1>
        <span className="rounded-lg border border-[#27272a] bg-[#18181b] px-3 py-1 text-sm text-[#a1a1aa]">
          Search only
        </span>
      </div>

      <SearchTab />
    </div>
  );
}
