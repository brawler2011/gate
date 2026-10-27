import {afterEach, describe, expect, it} from "bun:test";
import {getPathMatch} from "next/dist/shared/lib/router/utils/path-match";
import {NextRequest} from "next/server";

import {config, proxy} from "../proxy";

const originalBackendUrl = process.env.BACKEND_API_URL;
afterEach(() => {
  if (originalBackendUrl === undefined) {
    delete process.env.BACKEND_API_URL;
  } else {
    process.env.BACKEND_API_URL = originalBackendUrl;
  }
});

describe("API routing", () => {
  it("matches API paths including image extensions without matching unrelated assets", () => {
    for (const url of ["/api", "/api/posts/123/image", "/api/files/cover.png"]) {
      expect(config.matcher.some((pattern) => getPathMatch(pattern)(url))).toBe(true);
    }
    for (const url of ["/_next/image", "/_next/static/test.js", "/cover.png"]) {
      expect(config.matcher.some((pattern) => getPathMatch(pattern)(url))).toBe(false);
    }
  });

  it("rewrites to the runtime backend URL, preserving paths and query parameters", () => {
    for (const baseUrl of ["http://backend.test:8080", "http://backend.test:8080/"]) {
      process.env.BACKEND_API_URL = baseUrl;
      const response = proxy(new NextRequest("https://gate.test/api/posts/123/image?size=100&tag=a%2Fb&tag=c"));
      expect(response.headers.get("x-middleware-rewrite")).toBe("http://backend.test:8080/posts/123/image?size=100&tag=a%2Fb&tag=c");
    }
    process.env.BACKEND_API_URL = "http://other-backend.test/v1/";
    expect(proxy(new NextRequest("https://gate.test/api/posts")).headers.get("x-middleware-rewrite"))
      .toBe("http://other-backend.test/v1/posts");
  });

  it("requires backend configuration without redirecting API-like page paths", () => {
    delete process.env.BACKEND_API_URL;
    expect(() => proxy(new NextRequest("https://gate.test/api/posts"))).toThrow("BACKEND_API_URL");
    expect(proxy(new NextRequest("https://gate.test/apiary")).headers.get("x-middleware-next")).toBe("1");
  });

  it("preserves profile rewrites and direct internal profile blocking", () => {
    expect(proxy(new NextRequest("https://gate.test/@alice")).headers.get("x-middleware-rewrite"))
      .toBe("https://gate.test/user/alice");
    expect(proxy(new NextRequest("https://gate.test/user/alice")).status).toBe(404);
  });
});
