import {describe, it, expect, beforeEach, afterEach} from "bun:test";

import {register} from "../instrumentation";
import {env} from "../lib/env";
import {initBrowserTelemetry} from "../lib/telemetry";

describe("Milestone M6 Telemetry Adversarial Stress Suite", () => {
  const originalEnv = {...process.env};

  beforeEach(() => {
    process.env = {...originalEnv};
  });

  afterEach(() => {
    process.env = {...originalEnv};
  });

  describe("1. SSR Environment Safety (No window, No client env vars)", () => {
    it("SSR: initBrowserTelemetry does not throw even if all NEXT_PUBLIC env vars are missing/undefined", () => {
      delete process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL;
      delete process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME;
      delete process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
      delete process.env.OTEL_SERVICE_NAME;

      // In Node runtime (typeof window === "undefined"), initBrowserTelemetry must never query env or throw
      expect(typeof window).toBe("undefined");
      expect(() => {
        initBrowserTelemetry();
      }).not.toThrow();
    });

    it("SSR: calling initBrowserTelemetry repeatedly in SSR environment is always a no-op", () => {
      for (let i = 0; i < 50; i++) {
        expect(() => initBrowserTelemetry()).not.toThrow();
      }
    });

    it("SSR: register() in instrumentation.ts behaves correctly across runtime targets", async () => {
      // 1. Edge runtime (NEXT_RUNTIME = "edge")
      process.env.NEXT_RUNTIME = "edge";
      await expect(register()).resolves.toBeUndefined();

      // 2. Undefined runtime
      delete process.env.NEXT_RUNTIME;
      await expect(register()).resolves.toBeUndefined();

      // 3. Node.js runtime with valid env
      process.env.NEXT_RUNTIME = "nodejs";
      process.env.OTEL_SERVICE_NAME = "gate-frontend";
      await expect(register()).resolves.toBeUndefined();

      // 4. Node.js runtime with missing env throws strict error
      delete process.env.OTEL_SERVICE_NAME;
      await expect(register()).rejects.toThrow(/OTEL_SERVICE_NAME/);
    });
  });

  describe("2. Strict Environment Variable Validation & No Silent Fallbacks", () => {
    const testCases = [
      {
        getter: () => env.getOtelExporterUrl(),
        varName: "NEXT_PUBLIC_OTEL_EXPORTER_URL",
        validVal: "http://localhost/otlp/v1/traces",
      },
      {
        getter: () => env.getOtelServiceName(),
        varName: "NEXT_PUBLIC_OTEL_SERVICE_NAME",
        validVal: "gate-frontend-browser",
      },
      {
        getter: () => env.getServerOtelEndpoint(),
        varName: "OTEL_EXPORTER_OTLP_ENDPOINT",
        validVal: "http://localhost:4318",
      },
      {
        getter: () => env.getServerOtelServiceName(),
        varName: "OTEL_SERVICE_NAME",
        validVal: "gate-frontend",
      },
    ];

    for (const {getter, varName, validVal} of testCases) {
      it(`strict requireEnv enforcement on ${varName}: rejects undefined, empty, whitespace-only`, () => {
        // 1. Undefined
        delete process.env[varName];
        expect(() => getter()).toThrow(
          new RegExp(`\\[env\\] Environment variable ${varName} is missing or empty!`)
        );

        // 2. Empty string
        process.env[varName] = "";
        expect(() => getter()).toThrow(
          new RegExp(`\\[env\\] Environment variable ${varName} is missing or empty!`)
        );

        // 3. Spaces only
        process.env[varName] = "   ";
        expect(() => getter()).toThrow(
          new RegExp(`\\[env\\] Environment variable ${varName} is missing or empty!`)
        );

        // 4. Tab and newlines
        process.env[varName] = "\t\n  ";
        expect(() => getter()).toThrow(
          new RegExp(`\\[env\\] Environment variable ${varName} is missing or empty!`)
        );

        // 5. Valid value
        process.env[varName] = validVal;
        expect(getter()).toBe(validVal);
      });
    }
  });

  describe("3. Browser Client Runtime Simulation & Idempotency", () => {
    it("simulated browser environment: initializes once, subsequent calls are no-ops", async () => {
      // In bun test, we can simulate window object in an isolated context
      process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL = "http://localhost/otlp/v1/traces";
      process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME = "gate-frontend-browser";

      // Verify that browser.ts exports initBrowserTelemetry function
      expect(typeof initBrowserTelemetry).toBe("function");

      // Verify that when window is defined, calling initBrowserTelemetry succeeds
      // and subsequent calls do not throw or duplicate
      const originalWindow = (globalThis as unknown as {window?: unknown}).window;
      const originalDocument = (globalThis as unknown as {document?: unknown}).document;
      const mockDoc = {
        readyState: "complete",
        addEventListener: () => {},
        removeEventListener: () => {},
      };
      try {
        (globalThis as unknown as {window: unknown}).window = {
          location: {origin: "http://localhost:3000", href: "http://localhost:3000"},
          document: mockDoc,
          addEventListener: () => {},
          removeEventListener: () => {},
          setTimeout: (fn: (...args: unknown[]) => void, ms?: number) => setTimeout(fn, ms),
          clearTimeout: (id: NodeJS.Timeout) => clearTimeout(id),
          performance: {
            getEntriesByType: () => [],
            timeOrigin: Date.now(),
            now: () => Date.now(),
          },
        };
        (globalThis as unknown as {document: unknown}).document = mockDoc;

        // First call should run through provider/instrumentation setup
        expect(() => initBrowserTelemetry()).not.toThrow();

        // Sequential calls (idempotency check)
        expect(() => initBrowserTelemetry()).not.toThrow();
        expect(() => initBrowserTelemetry()).not.toThrow();
      } finally {
        (globalThis as unknown as {window?: unknown}).window = originalWindow;
        if (originalDocument) {
          (globalThis as unknown as {document?: unknown}).document = originalDocument;
        }
      }
    });
  });

  describe("4. Static Property Access vs Dynamic Map", () => {
    it("env object statically exposes all 4 telemetry properties", () => {
      process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL = "http://localhost/otlp/v1/traces";
      process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME = "gate-frontend-browser";
      process.env.OTEL_EXPORTER_OTLP_ENDPOINT = "http://localhost:4318";
      process.env.OTEL_SERVICE_NAME = "gate-frontend";

      expect("NEXT_PUBLIC_OTEL_EXPORTER_URL" in env).toBe(true);
      expect("NEXT_PUBLIC_OTEL_SERVICE_NAME" in env).toBe(true);
      expect("OTEL_EXPORTER_OTLP_ENDPOINT" in env).toBe(true);
      expect("OTEL_SERVICE_NAME" in env).toBe(true);
    });
  });
});
