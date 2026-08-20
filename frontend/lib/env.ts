const requireEnv = (name: string, value: string | undefined): string => {
  if (!value || value.trim() === "") {
    throw new Error(`[env] Environment variable ${name} is missing or empty!`);
  }
  return value;
};

export const env = {
  /**
   * Public backend API URL accessible in browser environment.
   */
  NEXT_PUBLIC_BACKEND_API_URL: process.env.NEXT_PUBLIC_BACKEND_API_URL,

  /**
   * Backend API URL accessible on server side (Server Components / Actions).
   */
  BACKEND_API_URL: process.env.BACKEND_API_URL,

  /**
   * WebSocket base URL for real-time updates.
   */
  NEXT_PUBLIC_WEBSOCKET_URL: process.env.NEXT_PUBLIC_WEBSOCKET_URL,

  /**
   * Public App URL.
   */
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,

  /**
   * Browser OTLP trace exporter URL.
   */
  NEXT_PUBLIC_OTEL_EXPORTER_URL: process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL,

  /**
   * Browser OTel service name.
   */
  NEXT_PUBLIC_OTEL_SERVICE_NAME: process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME,

  /**
   * Server OTLP collector endpoint.
   */
  OTEL_EXPORTER_OTLP_ENDPOINT: process.env.OTEL_EXPORTER_OTLP_ENDPOINT,

  /**
   * Server OTel service name.
   */
  OTEL_SERVICE_NAME: process.env.OTEL_SERVICE_NAME,

  /**
   * Node execution environment.
   */
  NODE_ENV: process.env.NODE_ENV,

  /**
   * Flag indicating production environment.
   */
  isProduction: (): boolean => {
    return process.env.NODE_ENV === "production";
  },

  /**
   * Flag indicating development environment.
   */
  isDevelopment: (): boolean => {
    return process.env.NODE_ENV === "development";
  },

  /**
   * Returns backend API URL based on client/server runtime context.
   */
  getBackendApiUrl: (): string => {
    if (typeof window !== "undefined") {
      return requireEnv("NEXT_PUBLIC_BACKEND_API_URL", process.env.NEXT_PUBLIC_BACKEND_API_URL);
    }
    return requireEnv("BACKEND_API_URL", process.env.BACKEND_API_URL);
  },

  /**
   * Returns WebSocket URL without trailing slashes.
   */
  getWebSocketUrl: (): string => {
    return requireEnv("NEXT_PUBLIC_WEBSOCKET_URL", process.env.NEXT_PUBLIC_WEBSOCKET_URL).replace(/\/+$/, "");
  },

  /**
   * Returns public App URL without trailing slashes.
   */
  getAppUrl: (): string => {
    return requireEnv("NEXT_PUBLIC_APP_URL", process.env.NEXT_PUBLIC_APP_URL).replace(/\/+$/, "");
  },

  /**
   * Returns browser OTLP exporter URL.
   */
  getOtelExporterUrl: (): string => {
    return requireEnv("NEXT_PUBLIC_OTEL_EXPORTER_URL", process.env.NEXT_PUBLIC_OTEL_EXPORTER_URL);
  },

  /**
   * Returns browser OTel service name.
   */
  getOtelServiceName: (): string => {
    return requireEnv("NEXT_PUBLIC_OTEL_SERVICE_NAME", process.env.NEXT_PUBLIC_OTEL_SERVICE_NAME);
  },

  /**
   * Returns server OTLP collector endpoint.
   */
  getServerOtelEndpoint: (): string => {
    return requireEnv("OTEL_EXPORTER_OTLP_ENDPOINT", process.env.OTEL_EXPORTER_OTLP_ENDPOINT);
  },

  /**
   * Returns server OTel service name.
   */
  getServerOtelServiceName: (): string => {
    return requireEnv("OTEL_SERVICE_NAME", process.env.OTEL_SERVICE_NAME);
  },
} as const;
