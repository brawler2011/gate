"use server";

import { cookies } from "next/headers";
import { core } from "@contracts/core/v1";
import { ApiError as CoreApiError } from "@contracts/core/v1/core/ApiError";

const sessionCookieName = "session_id";

/**
 * API error info returned from Call
 */
export type ApiError = {
  status: number;
  message: string;
  requestId?: string;
};

const getSessionCookie = async (): Promise<string | undefined> => {
  const requestCookies = await cookies();

  if (!requestCookies.has(sessionCookieName)) {
    return;
  }

  const cookie = requestCookies.get(sessionCookieName);

  if (!cookie || !cookie.name || !cookie.value) {
    return;
  }

  return `${sessionCookieName}=${cookie.value}`;
};

/**
 * Call unified Core API method and return [error, data] tuple
 * Returns [null, data] on success, [error, null] on failure
 */
export const Call = async <T>(
  method: (client: core) => Promise<T>
): Promise<[ApiError | null, T | null]> => {
  const headers: Record<string, string> = {};

  const sessionCookie = await getSessionCookie();

  if (sessionCookie) {
    headers["Cookie"] = sessionCookie;
  }

  const client = new core({
    BASE: process.env.BACKEND_API_URL,
    HEADERS: headers,
    CREDENTIALS: "include",
  });

  try {
    const data = await method(client);
    return [null, data];
  } catch (error) {
    if (error instanceof CoreApiError) {
      // Don't log 401 errors - they're expected when user is not authenticated
      if (error.status !== 401) {
        console.log("error", error);
      }
      const body = error.body as { message?: string; request_id?: string } | undefined;
      return [{
        status: error.status,
        message: body?.message || error.statusText,
        requestId: body?.request_id,
      }, null];
    }
    console.error('Unknown error:', error);
    return [{ status: 500, message: 'Неизвестная ошибка' }, null];
  }
};
