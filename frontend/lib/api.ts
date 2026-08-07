import { core, type AuthService, type DefaultService } from "@/contracts/core/v1";
import { ApiError as CoreApiError } from "@/contracts/core/v1/core/ApiError";
import { env } from "@/lib/env";

const sessionCookieName = "session_id";

const getBaseUrl = () => {
  return env.getBackendApiUrl() ?? "";
};

/**
 * API error info returned from methods
 */
export type ApiError = {
  status: number;
  message: string;
  requestId?: string;
};

// Merged type of all services, converting CancelablePromise<T> to Promise<[ApiError | null, T | null]>
type MergedServices = DefaultService & AuthService;

export type ApiFacade = {
  [K in keyof MergedServices]: MergedServices[K] extends (...args: infer A) => Promise<infer R>
    ? (...args: A) => Promise<[ApiError | null, R | null]>
    : MergedServices[K];
};

function createApiFacade(client: core): ApiFacade {
  return new Proxy(client, {
    get(target, prop: string) {
      // Find matching method on auth or default service
      const service = [target.auth, target.default].find(
        (s) => typeof (s as any)[prop] === "function"
      );
      const fn = service ? (service as any)[prop] : (target as any)[prop];

      if (typeof fn === "function") {
        return async (...args: any[]) => {
          try {
            const data = await fn.apply(service || target, args);
            return [null, data];
          } catch (error) {
            if (error instanceof CoreApiError) {
              if (error.status !== 401) {
                console.log("API Error:", error);
              }
              const body = error.body as { message?: string; request_id?: string } | undefined;
              return [{
                status: error.status,
                message: body?.message || error.statusText,
                requestId: body?.request_id,
              }, null];
            }
            console.error("Unknown API Error:", error);
            return [{ status: 500, message: "Неизвестная ошибка" }, null];
          }
        };
      }
      return fn;
    },
  }) as unknown as ApiFacade;
}

const rawAuthClient = new core({
  BASE: getBaseUrl(),
  CREDENTIALS: "include",
  HEADERS: async (): Promise<Record<string, string>> => {
    if (typeof window === "undefined") {
      try {
        const { cookies } = await import("next/headers");
        const requestCookies = await cookies();
        if (requestCookies.has(sessionCookieName)) {
          const cookie = requestCookies.get(sessionCookieName);
          if (cookie?.name && cookie?.value) {
            return { Cookie: `${sessionCookieName}=${cookie.value}` };
          }
        }
      } catch {
        // Outside HTTP request context
      }
    }
    return {};
  },
});

const rawPublicClient = new core({
  BASE: getBaseUrl(),
  CREDENTIALS: "omit",
});

/**
 * Primary authenticated API facade singleton.
 * Direct method access without .default or .auth, returning [error, data] tuples.
 */
export const api = createApiFacade(rawAuthClient);

/**
 * Public API facade singleton without session cookies for SSG / ISR.
 */
export const publicApi = createApiFacade(rawPublicClient);
