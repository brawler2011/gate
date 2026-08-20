import {describe, it, expect, beforeEach, afterEach} from "bun:test";

import {env} from "../lib/env";
import {initBrowserTelemetry} from "../lib/telemetry";

describe("OpenTelemetry Telemetry Suite", () => {
  const originalEnv = {...process.env};

  beforeEach(() => {
    process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL = "http://localhost/otlp/v1/traces";
    process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME = "gate-frontend-browser";
    process.env.OTEL_EXPORTER_OTLP_ENDPOINT = "http://localhost:4318";
    process.env.OTEL_SERVICE_NAME = "gate-frontend";
  });

  afterEach(() => {
    process.env = {...originalEnv};
  });

  describe("1. Environment Variable Accessors", () => {
    it("returns correct telemetry environment variable values", () => {
      expect(env.getOtelExporterUrl()).toBe("http://localhost/otlp/v1/traces");
      expect(env.getOtelServiceName()).toBe("gate-frontend-browser");
      expect(env.getServerOtelEndpoint()).toBe("http://localhost:4318");
      expect(env.getServerOtelServiceName()).toBe("gate-frontend");
    });

    it("throws an error when required telemetry variable is empty or missing", () => {
      delete process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL;
      expect(() => env.getOtelExporterUrl()).toThrow(/NEXT_PUBLIC_OTEL_EXPORTER_URL/);

      delete process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME;
      expect(() => env.getOtelServiceName()).toThrow(/NEXT_PUBLIC_OTEL_SERVICE_NAME/);

      delete process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
      expect(() => env.getServerOtelEndpoint()).toThrow(/OTEL_EXPORTER_OTLP_ENDPOINT/);

      delete process.env.OTEL_SERVICE_NAME;
      expect(() => env.getServerOtelServiceName()).toThrow(/OTEL_SERVICE_NAME/);
    });
  });

  describe("2. Browser Telemetry SSR Safety & Idempotency", () => {
    it("does not throw when executed in Node/SSR environment (window is undefined)", () => {
      expect(() => {
        initBrowserTelemetry();
      }).not.toThrow();
    });

    it("safely handles multiple invocations (idempotent)", () => {
      expect(() => {
        initBrowserTelemetry();
        initBrowserTelemetry();
        initBrowserTelemetry();
      }).not.toThrow();
    });
  });
});
