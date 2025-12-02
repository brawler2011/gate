"use server";

import { cookies } from "next/headers";
import { core } from "../../contracts/core/v1";
import { ApiError as ContractsApiError } from "../../contracts/core/v1/core/ApiError";

const oryKratosCookieName = "ory_kratos_session";

/**
 * API error info returned from Call
 */
export type ApiError = {
  status: number;
  message: string;
  requestId?: string;
};

const getKratosCookie = async (): Promise<string | undefined> => {
  const requestCookies = await cookies();

  if (!requestCookies.has(oryKratosCookieName)) {
    return;
  }

  const cookie = requestCookies.get(oryKratosCookieName);

  if (!cookie || !cookie.name || !cookie.value) {
    return;
  }

  return `${oryKratosCookieName}=${cookie.value}`;
};

/**
 * Call API method and return [error, data] tuple
 * Returns [null, data] on success, [error, null] on failure
 */
export const Call = async <T>(
  method: (client: core) => Promise<T>
): Promise<[ApiError | null, T | null]> => {
  const headers: Record<string, string> = {};

  const kratosCookie = await getKratosCookie();

  headers["Cookie"] = kratosCookie || "";

  const client = new core({
    BASE: process.env.BACKEND_API_URL,
    HEADERS: headers,
    CREDENTIALS: "include",
  });

  try {
    const data = await method(client);
    return [null, data];
  } catch (error) {
    if (error instanceof ContractsApiError) {
      const body = error.body as { message?: string; request_id?: string } | undefined;
      return [{
        status: error.status,
        message: body?.message || error.statusText,
        requestId: body?.request_id,
      }, null];
    }
    return [{ status: 500, message: 'Неизвестная ошибка' }, null];
  }
};
