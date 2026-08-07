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
  WEBSOCKET_URL: process.env.WEBSOCKET_URL,

  /**
   * Public App URL.
   */
  NEXT_PUBLIC_APP_URL: process.env.NEXT_PUBLIC_APP_URL,

  /**
   * Node execution environment.
   */
  NODE_ENV: process.env.NODE_ENV,

  /**
   * Flag indicating production environment.
   */
  get isProduction(): boolean {
    return process.env.NODE_ENV === "production";
  },

  /**
   * Flag indicating development environment.
   */
  get isDevelopment(): boolean {
    return process.env.NODE_ENV === "development";
  },

  /**
   * Returns backend API URL based on client/server runtime context.
   */
  getBackendApiUrl(): string | undefined {
    if (typeof window !== "undefined") {
      return process.env.NEXT_PUBLIC_BACKEND_API_URL;
    }
    return process.env.BACKEND_API_URL;
  },

  /**
   * Returns WebSocket URL without trailing slashes.
   */
  getWebSocketUrl(): string | undefined {
    return process.env.WEBSOCKET_URL?.replace(/\/+$/, "");
  },
} as const;
