export function parseNodeTags(tags: string): string[] {
  try {
    const parsed = JSON.parse(tags);
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.map((value) => String(value).trim()).filter(Boolean);
  } catch {
    return [];
  }
}

export function getNodeDisplayName(hostname: string, tags: string): string {
  return parseNodeTags(tags)[0] || hostname;
}
