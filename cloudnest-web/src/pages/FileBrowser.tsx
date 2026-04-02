import { useState, useEffect, useRef, useCallback } from "react";
import { Search, FileText, Loader2, FolderSearch, Download } from "lucide-react";
import { searchFiles, getNodeDownloadURL, type SearchResult } from "../api/client";

function formatBytes(bytes: number, decimals = 1): string {
  if (!bytes || bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + " " + sizes[i];
}

export default function FileBrowser() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResults([]);
      setSearched(false);
      return;
    }
    setLoading(true);
    setSearched(true);
    try {
      const data = await searchFiles(q.trim());
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
    return () => { if (timerRef.current) clearTimeout(timerRef.current); };
  }, [query, doSearch]);

  const handleDownload = async (nodeUuid: string, path: string) => {
    try {
      const res = await getNodeDownloadURL(nodeUuid, path);
      window.open(res.url, "_blank");
    } catch { /* ignore */ }
  };

  // Group by node_uuid
  const grouped = results.reduce<Record<string, SearchResult[]>>((acc, r) => {
    (acc[r.node_uuid] ||= []).push(r);
    return acc;
  }, {});

  return (
    <div className="space-y-6 animate-[fadeIn_0.3s_ease-out]">
      <div>
        <h1 className="text-xl font-bold text-[#fafafa] mb-4">File Search</h1>
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
                <span className="text-xs font-mono text-[#a1a1aa]">
                  Node: {nodeUuid.slice(0, 12)}...
                </span>
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
