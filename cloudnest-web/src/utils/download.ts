export function triggerDownload(url: string, filename?: string): void {
  const link = document.createElement("a");
  link.href = url;
  link.rel = "noopener";
  link.download = filename && filename.trim() ? filename.trim() : "";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

export function inferFilenameFromPath(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/\/+$/, "");
  if (!normalized || normalized === "/") return "download";
  const parts = normalized.split("/");
  const name = parts[parts.length - 1];
  return name || "download";
}
