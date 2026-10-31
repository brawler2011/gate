import {QueryClient} from "@tanstack/react-query";
import {afterEach, expect, it, spyOn} from "bun:test";

const originalEnv = {...process.env};
const originalFetch = globalThis.fetch;
afterEach(() => {
  process.env = {...originalEnv}; globalThis.fetch = originalFetch;
});

it("API errors retain requestId and log only safe operation metadata", async () => {
  process.env.BACKEND_API_URL = "http://backend.test";
  const {api} = await import("../lib/api");
  const logged = spyOn(console, "error").mockImplementation(() => {});
  try {
    globalThis.fetch = (async () => new Response(JSON.stringify({
      message: "body-secret", request_id: "request-123", token: "token-secret",
    }), {status: 500, statusText: "status-secret", headers: {"Content-Type": "application/json"}})) as unknown as typeof fetch;
    const {error} = await api.GET("/users/me");
    expect((error as unknown as {request_id?: string})?.request_id).toBe("request-123");
    expect(logged.mock.calls).toEqual([["API Error:", {operation: "/users/me", status: 500, requestId: "request-123", errorType: "ApiError"}]]);
    expect(JSON.stringify(logged.mock.calls)).not.toContain("secret");
    logged.mockClear();
    globalThis.fetch = (async () => {
      throw new Error("network-secret");
    }) as unknown as typeof fetch;
    const failed = await api.GET("/users/me");
    expect(failed.response.status).toBe(500);
    expect(failed.error).toMatchObject({status: 500, message: "Не удалось выполнить запрос"});
    expect(logged.mock.calls).toEqual([["API Error:", {operation: "/users/me", status: 500, requestId: undefined, errorType: "ApiError"}]]);
  } finally {
    logged.mockRestore();
  }
});

for (const status of [401, 403, 404, 500]) {
  it(`query errors retain HTTP status ${status} and request metadata`, async () => {
    const {api} = await import("../lib/api");
    const logged = spyOn(console, "error").mockImplementation(() => {});
    const client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
    try {
      globalThis.fetch = (async () => Response.json({message: "Access denied", request_id: "req-1"}, {
        status, headers: {"Content-Length": "1", "X-Request-Id": "req-1"},
      })) as unknown as typeof fetch;
      await expect(client.fetchQuery(api.queryOptions("get", "/users/me"))).rejects.toMatchObject({
        status, message: "Access denied", request_id: "req-1", requestId: "req-1",
      });
      const {response} = await api.GET("/users/me");
      expect(response.status).toBe(status);
      expect(response.headers.get("X-Request-Id")).toBe("req-1");
      expect(response.headers.has("Content-Length")).toBe(false);
    } finally {
      client.clear();
      logged.mockRestore();
    }
  });
}

it("normalizes non-JSON HTTP errors for query consumers", async () => {
  const {api} = await import("../lib/api");
  const logged = spyOn(console, "error").mockImplementation(() => {});
  const client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
  try {
    globalThis.fetch = (async () => new Response("proxy error", {status: 502, statusText: "Bad Gateway"})) as unknown as typeof fetch;
    await expect(client.fetchQuery(api.queryOptions("get", "/users/me"))).rejects.toMatchObject({
      status: 502, message: "Bad Gateway",
    });
  } finally {
    client.clear();
    logged.mockRestore();
  }
});

it("keeps cancellation as an exception", async () => {
  const {api} = await import("../lib/api");
  globalThis.fetch = (async () => {
    throw new DOMException("Aborted", "AbortError");
  }) as unknown as typeof fetch;
  await expect(api.GET("/users/me")).rejects.toMatchObject({name: "AbortError"});
});

it("returns network errors for mutations and throws normalized errors in queries", async () => {
  const {api} = await import("../lib/api");
  const logged = spyOn(console, "error").mockImplementation(() => {});
  const client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
  try {
    globalThis.fetch = (async () => {
      throw new TypeError("Failed to fetch");
    }) as unknown as typeof fetch;
    const {error, response} = await api.POST("/organizations/{login}/requests", {
      params: {path: {login: "school"}},
    });
    expect(response.ok).toBe(false);
    expect(error).toMatchObject({status: 500, message: "Не удалось выполнить запрос"});
    await expect(client.fetchQuery(api.queryOptions("get", "/users/me"))).rejects.toMatchObject({
      status: 500, message: "Не удалось выполнить запрос",
    });
  } finally {
    client.clear();
    logged.mockRestore();
  }
});
