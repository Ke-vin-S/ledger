// Helpers for the post-auth "next" redirect target. Only same-origin relative
// paths are honoured, to avoid open-redirect vulnerabilities.

export function safeNextPath(raw: string | null | undefined): string | null {
  if (!raw) return null;
  // Must be a relative path ("/foo"), not a protocol-relative ("//host") or absolute URL.
  if (!raw.startsWith("/") || raw.startsWith("//")) return null;
  return raw;
}

export function readNextParam(): string | null {
  if (typeof window === "undefined") return null;
  return safeNextPath(new URLSearchParams(window.location.search).get("next"));
}
