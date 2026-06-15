"use server";

import { cookies } from "next/headers";
import { Call } from "./api";

export type AuthActionResult = {
  success: boolean;
  error?: string;
};

export async function loginAction(identifier: string, password: string): Promise<AuthActionResult> {
  const [error, response] = await Call((client) =>
    client.auth.login({
      requestBody: { identifier, password },
    })
  );

  if (error || !response) {
    return {
      success: false,
      error: error?.message || "Неверный логин или пароль",
    };
  }

  const cookieStore = await cookies();
  cookieStore.set("session_id", response.session_id, {
    path: "/",
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    maxAge: 7 * 24 * 60 * 60, // 7 days (sliding expiry is handled on backend, but we match cookie)
  });

  return { success: true };
}

export async function registerAction(username: string, email: string, password: string): Promise<AuthActionResult> {
  const [error, response] = await Call((client) =>
    client.auth.register({
      requestBody: { username, email, password },
    })
  );

  if (error || !response) {
    return {
      success: false,
      error: error?.message || "Ошибка при регистрации",
    };
  }

  const cookieStore = await cookies();
  cookieStore.set("session_id", response.session_id, {
    path: "/",
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    maxAge: 7 * 24 * 60 * 60,
  });

  return { success: true };
}

export async function logoutAction(): Promise<AuthActionResult> {
  // Let the backend delete the session first
  const [error] = await Call((client) => client.auth.logout());

  // Clear cookie regardless of whether backend call succeeded
  const cookieStore = await cookies();
  cookieStore.set("session_id", "", { path: "/", maxAge: -1 });

  if (error) {
    return {
      success: false,
      error: error.message,
    };
  }

  return { success: true };
}
