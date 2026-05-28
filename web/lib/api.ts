import { getAccessToken, setAccessToken } from "./auth";
import { API_BASE_URL } from "@/constants/config";

const BASE = API_BASE_URL;

type ApiError = {
  code: string;
  message: string;
  field?: string;
};

export class ApiRequestError extends Error {
  constructor(
    public readonly status: number,
    public readonly error: ApiError,
  ) {
    super(error.message);
    this.name = "ApiRequestError";
  }
}

async function parseError(res: Response): Promise<ApiRequestError> {
  try {
    const body = await res.json();
    return new ApiRequestError(res.status, body.error ?? { code: "UNKNOWN", message: res.statusText });
  } catch {
    return new ApiRequestError(res.status, { code: "UNKNOWN", message: res.statusText });
  }
}

// Only one refresh in-flight at a time — prevents race when multiple 401s fire simultaneously.
let refreshPromise: Promise<boolean> | null = null;

async function refreshTokens(): Promise<boolean> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    try {
      const res = await fetch(`${BASE}/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) return false;
      const body = await res.json();
      setAccessToken(body.data?.access_token ?? body.access_token);
      return true;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

async function request<T>(
  path: string,
  init: RequestInit = {},
  retry = true,
): Promise<T> {
  const token = getAccessToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(init.headers as Record<string, string>),
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (res.status === 401 && retry) {
    const refreshed = await refreshTokens();
    if (refreshed) return request<T>(path, init, false);
    // Refresh failed — redirect to login
    if (typeof window !== "undefined") {
      window.location.href = "/login"; // avoid circular import — ROUTES not used here
    }
    throw new ApiRequestError(401, { code: "UNAUTHORIZED", message: "Session expired" });
  }

  if (!res.ok) throw await parseError(res);

  if (res.status === 204) return undefined as T;
  // All backend responses are enveloped as { data: T, meta: { ... } }. Unwrap data.
  const body = await res.json();
  return (body.data !== undefined ? body.data : body) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: "GET" }),

  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PATCH",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  put: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "PUT",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  delete: <T = void>(path: string) => request<T>(path, { method: "DELETE" }),
};
