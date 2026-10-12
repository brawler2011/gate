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
    const [error] = await api.getMe();
    expect(error?.requestId).toBe("request-123");
    expect(logged.mock.calls).toEqual([["API Error:", {operation: "getMe", status: 500, requestId: "request-123", errorType: "ApiError"}]]);
    expect(JSON.stringify(logged.mock.calls)).not.toContain("secret");
    logged.mockClear();
    globalThis.fetch = (async () => {
      throw new Error("network-secret");
    }) as unknown as typeof fetch;
    await api.getMe();
    expect(logged.mock.calls).toEqual([["Unknown API Error:", {operation: "getMe", status: 500, errorType: "Error"}]]);
  } finally {
    logged.mockRestore();
  }
});
