"use server";

import { cookies } from "next/headers";
import { core } from "../../contracts/core/v1";
import { ApiError } from "../../contracts/core/v1/core/ApiError";

const oryKratosCookieName = "ory_kratos_session";

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

export const Call = async <T>(
  method: (client: core) => Promise<T>
): Promise<T> => {
  const headers: Record<string, string> = {};

  const kratosCookie = await getKratosCookie();

  headers["Cookie"] = kratosCookie || "";

  const client = new core({
    BASE: process.env.BACKEND_API_URL,
    HEADERS: headers,
    CREDENTIALS: "include",
  });

  try {
    return await method(client);
  } catch (error) {
    // Re-throw with informative message that survives SSR serialization
    if (error instanceof ApiError) {
      const body = error.body as { message?: string; request_id?: string } | undefined;
      const details = body?.message || error.statusText;
      const requestId = body?.request_id ? ` [${body.request_id}]` : "";
      throw new Error(`${error.status}: ${details}${requestId}`);
    }
    throw error;
  }
};
