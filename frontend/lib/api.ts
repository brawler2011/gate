"use server";

import { cookies } from "next/headers";
import { gateway } from "@contracts/gateway/v1";
import { ApiError as GatewayApiError } from "@contracts/gateway/v1/core/ApiError";

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
 * Call unified Gateway API method and return [error, data] tuple
 * Returns [null, data] on success, [error, null] on failure
 * 
 * Gateway API combines all microservices (blogs, tester/core) into one client.
 * NGINX routes requests to the appropriate microservice based on the path.
 */
export const Call = async <T>(
  method: (client: gateway) => Promise<T>
): Promise<[ApiError | null, T | null]> => {
  const headers: Record<string, string> = {};

  const kratosCookie = await getKratosCookie();

  if (kratosCookie) {
    headers["Cookie"] = kratosCookie;
  }

  const client = new gateway({
    BASE: process.env.BACKEND_API_URL,
    HEADERS: headers,
    CREDENTIALS: "include",
  });

  try {
    const data = await method(client);
    return [null, data];
  } catch (error) {
    if (error instanceof GatewayApiError) {
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
