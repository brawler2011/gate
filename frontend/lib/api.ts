import createFetchClient from "openapi-fetch";
import createClient from "openapi-react-query";
import {redirect} from "react-router-dom";

import type {paths} from "@/contracts/core/v1/schema";
import type {Client, MaybeOptionalInit, ParseAsResponse} from "openapi-fetch";
import type {
  ErrorResponse,
  HttpMethod,
  MediaType,
  PathsWithMethod,
  Readable,
  RequiredKeysOf,
  SuccessResponse,
} from "openapi-typescript-helpers";

export type ApiError = {status: number; message: string; requestId?: string};

class ApiRequest {
  url: string;
  method: string;
  headers: Headers;
  init?: RequestInit;
  body?: BodyInit | null;
  signal?: AbortSignal | null;

  constructor(input: string | {url: string}, init?: RequestInit) {
    this.url = typeof input === "string" ? input : input.url;
    this.method = init?.method || "GET";
    this.headers = new Headers(init?.headers);
    this.init = init;
    this.body = init?.body;
    this.signal = init?.signal;
  }

  clone(): ApiRequest {
    return new ApiRequest(this.url, this.init);
  }
}

const fetchClient = createFetchClient<paths>({
  baseUrl: "/api",
  credentials: "include",
  Request: ApiRequest as unknown as typeof Request,
  bodySerializer: (body) => {
    if (
      (body as unknown) instanceof Blob ||
      (body as unknown) instanceof ArrayBuffer ||
      ArrayBuffer.isView(body) ||
      (body as unknown) instanceof FormData
    ) {
      return body as unknown as BodyInit;
    }
    return JSON.stringify(body);
  },
  fetch: (req: unknown, init?: RequestInit) => {
    const request = req as ApiRequest;
    const url = typeof req === "string" ? req : request.url;
    const options: RequestInit = {
      credentials: "include",
      signal: request.signal,
      method: request.method,
      headers: request.headers,
      body: request.body,
      ...init,
    };
    return globalThis.fetch(url, options);
  },
});

fetchClient.use({
  onRequest: async ({request}) => {
    const body = request.body as unknown;
    if (body instanceof Blob) {
      if (body.type) {
        request.headers.set("Content-Type", body.type);
      } else if (
        !request.headers.get("Content-Type") ||
        request.headers.get("Content-Type") === "application/json"
      ) {
        request.headers.set("Content-Type", "application/octet-stream");
      }
    } else if (body instanceof ArrayBuffer || ArrayBuffer.isView(body)) {
      if (
        !request.headers.get("Content-Type") ||
        request.headers.get("Content-Type") === "application/json"
      ) {
        request.headers.set("Content-Type", "application/octet-stream");
      }
    }
  },
  onResponse: async ({response, schemaPath}) => {
    if (!response.ok) {
      let info: {message?: string; request_id?: string} | undefined;
      try {
        const cloned = response.clone();
        info = await cloned.json();
      } catch {
        // ignore
      }
      const requestId = info?.request_id;
      if (response.status !== 401) {
        console.error("API Error:", {
          operation: schemaPath,
          status: response.status,
          requestId: requestId && /^[A-Za-z0-9._:-]{1,128}$/.test(requestId) ? requestId : undefined,
          errorType: "ApiError",
        });
      }
    }
  },
  onError: async ({error, schemaPath}) => {
    console.error("Unknown API Error:", {
      operation: schemaPath,
      status: 500,
      errorType: error instanceof Error ? "Error" : "Unknown",
    });
  },
});

type SafeError<E> = ([E] extends [never] ? object : E) & {
  message?: string;
  request_id?: string;
  [key: string]: unknown;
};

type SafeResponseMap<Op> = Op extends {responses: Record<string | number, unknown>}
  ? Op["responses"]
  : Record<string | number, unknown>;

type ApiResponse<Op, Init> =
  | {
      data: ParseAsResponse<
        Readable<SuccessResponse<SafeResponseMap<Op>, MediaType>>,
        Init
      >;
      error?: never;
      response: Response;
    }
  | {
      data?: never;
      error: SafeError<
        Readable<ErrorResponse<SafeResponseMap<Op>, MediaType>>
      >;
      response: Response;
    };

type InitParam<Init> =
  RequiredKeysOf<Init> extends never
    ? [(Init & {[key: string]: unknown})?]
    : [Init & {[key: string]: unknown}];

type ApiClientMethod<Method extends HttpMethod> = <
  Path extends PathsWithMethod<paths, Method>,
  Init extends MaybeOptionalInit<paths[Path], Method>,
>(
  url: Path,
  ...init: InitParam<Init>
) => Promise<ApiResponse<paths[Path][Method], Init>>;

interface ApiClient {
  GET: ApiClientMethod<"get">;
  POST: ApiClientMethod<"post">;
  PUT: ApiClientMethod<"put">;
  DELETE: ApiClientMethod<"delete">;
  PATCH: ApiClientMethod<"patch">;
  HEAD: ApiClientMethod<"head">;
  OPTIONS: ApiClientMethod<"options">;
  use: Client<paths>["use"];
  eject: Client<paths>["eject"];
}

export type Api = ReturnType<typeof createClient<paths>> & ApiClient;

export const api = Object.assign(createClient(fetchClient), fetchClient) as unknown as Api;

type FetchPromise<T> = Promise<{
  data?: T;
  error?: {message?: string; request_id?: string} | unknown;
  response: Response;
}>;

type UnwrapFn = {
  <T>(promise: FetchPromise<T>): Promise<T>;
  <A extends unknown[], T>(fn: (...args: A) => FetchPromise<T>): (...args: A) => Promise<T>;
};

export const unwrap: UnwrapFn = ((
  arg: FetchPromise<unknown> | ((...args: unknown[]) => FetchPromise<unknown>),
) => {
  if (typeof arg === "function") {
    return async (...args: unknown[]) => unwrap(arg(...args));
  }
  return (async () => {
    const {data, error, response} = await arg;
    if (!response.ok || error || !data) {
      const info = (error && typeof error === "object")
        ? error as {message?: string; request_id?: string}
        : undefined;
      const message = info?.message || response.statusText || "API Error";
      if (response.status === 404) {
        throw new Response(message, {status: 404});
      }
      if (response.status === 401) {
        throw redirect("/auth/login");
      }
      const err = new Error(message) as Error & {status?: number; info?: unknown; requestId?: string};
      err.status = response.status;
      err.info = error;
      err.requestId = info?.request_id;
      throw err;
    }
    return data;
  })();
}) as UnwrapFn;
