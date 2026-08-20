"use client";

import {ZoneContextManager} from "@opentelemetry/context-zone";
import {W3CTraceContextPropagator} from "@opentelemetry/core";
import {OTLPTraceExporter} from "@opentelemetry/exporter-trace-otlp-http";
import {registerInstrumentations} from "@opentelemetry/instrumentation";
import {DocumentLoadInstrumentation} from "@opentelemetry/instrumentation-document-load";
import {FetchInstrumentation} from "@opentelemetry/instrumentation-fetch";
import {resourceFromAttributes} from "@opentelemetry/resources";
import {BatchSpanProcessor} from "@opentelemetry/sdk-trace-base";
import {WebTracerProvider} from "@opentelemetry/sdk-trace-web";
import {ATTR_SERVICE_NAME} from "@opentelemetry/semantic-conventions";

import {env} from "@/lib/env";

let isInitialized = false;

/**
 * Initializes OpenTelemetry Web SDK for browser-side distributed tracing.
 * Idempotent: safe to call multiple times (e.g. React StrictMode).
 * SSR-safe: immediately no-ops if executed outside browser runtime.
 */
export const initBrowserTelemetry = (): void => {
  if (typeof window === "undefined" || isInitialized) {
    return;
  }
  isInitialized = true;

  const resource = resourceFromAttributes({
    [ATTR_SERVICE_NAME]: env.getOtelServiceName(),
  });

  const exporter = new OTLPTraceExporter({
    url: env.getOtelExporterUrl(),
  });

  const spanProcessor = new BatchSpanProcessor(exporter, {
    maxQueueSize: 100,
    maxExportBatchSize: 10,
    scheduledDelayMillis: 1000,
    exportTimeoutMillis: 30000,
  });

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [spanProcessor],
  });

  provider.register({
    contextManager: new ZoneContextManager(),
    propagator: new W3CTraceContextPropagator(),
  });

  registerInstrumentations({
    tracerProvider: provider,
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/.+/],
        clearTimingResources: true,
        ignoreUrls: [/\/otlp\/v1\/traces/],
      }),
    ],
  });
};
