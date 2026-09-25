import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { api, ApiRequestError, resetApiStateForTests } from "./api";
import { setAccessToken, getAccessToken } from "./auth";

// Minimal Response-like object matching what lib/api.ts reads (ok, status, json()).
function res(status: number, body?: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

describe("api request wrapper", () => {
  beforeEach(() => {
    setAccessToken(null);
    resetApiStateForTests();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("unwraps the { data } envelope on success", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => res(200, { data: { id: "1", name: "Trip" }, meta: {} })));
    const out = await api.get<{ id: string; name: string }>("/teams/1");
    expect(out).toEqual({ id: "1", name: "Trip" });
  });

  it("returns undefined for 204 No Content", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => res(204)));
    const out = await api.delete("/teams/1");
    expect(out).toBeUndefined();
  });

  it("attaches the bearer token when present", async () => {
    const fetchMock = vi.fn(async () => res(200, { data: true }));
    vi.stubGlobal("fetch", fetchMock);
    setAccessToken("my-token");

    await api.get("/users/me");

    const init = (fetchMock.mock.calls[0] as unknown[])[1] as RequestInit;
    expect((init.headers as Record<string, string>)["Authorization"]).toBe("Bearer my-token");
  });

  it("throws ApiRequestError carrying the server error envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => res(422, { error: { code: "INVALID_SPLIT_SUM", message: "bad split", field: "splits" } })),
    );

    await expect(api.post("/teams/1/expenses", {})).rejects.toMatchObject({
      status: 422,
      error: { code: "INVALID_SPLIT_SUM", field: "splits" },
    });
  });

  it("on 401 refreshes the token then retries the original request once", async () => {
    const calls: string[] = [];
    const fetchMock = vi.fn(async (url: string) => {
      calls.push(url);
      if (url.endsWith("/auth/refresh")) {
        return res(200, { data: { access_token: "fresh-token" } });
      }
      // First hit is unauthorized; the retry (after refresh) succeeds.
      const priorDataHits = calls.filter((u) => u.endsWith("/protected")).length;
      return priorDataHits >= 2 ? res(200, { data: "ok" }) : res(401);
    });
    vi.stubGlobal("fetch", fetchMock);

    const out = await api.get<string>("/protected");

    expect(out).toBe("ok");
    expect(getAccessToken()).toBe("fresh-token");
    expect(calls.filter((u) => u.endsWith("/auth/refresh"))).toHaveLength(1);
  });

  it("on failed refresh redirects to /login and throws 401", async () => {
    // Replace window.location so href assignment is observable and inert.
    const original = window.location;
    // @ts-expect-error redefining for test
    delete window.location;
    // @ts-expect-error minimal stub
    window.location = { href: "" };

    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => (url.endsWith("/auth/refresh") ? res(401) : res(401))),
    );

    await expect(api.get("/protected")).rejects.toBeInstanceOf(ApiRequestError);
    expect(window.location.href).toBe("/login");

    // @ts-expect-error restore
    window.location = original;
  });

  it("dedupes concurrent refreshes when several requests 401 at once", async () => {
    let refreshCount = 0;
    const dataHits: Record<string, number> = {};
    const fetchMock = vi.fn(async (url: string) => {
      if (url.endsWith("/auth/refresh")) {
        refreshCount++;
        return res(200, { data: { access_token: "shared-token" } });
      }
      dataHits[url] = (dataHits[url] ?? 0) + 1;
      // Each distinct path is 401 on first hit, ok on retry.
      return dataHits[url] >= 2 ? res(200, { data: url }) : res(401);
    });
    vi.stubGlobal("fetch", fetchMock);

    const [a, b] = await Promise.all([api.get<string>("/a"), api.get<string>("/b")]);

    expect(a).toContain("/a");
    expect(b).toContain("/b");
    // Both 401s should share a single in-flight refresh.
    expect(refreshCount).toBe(1);
  });
});
