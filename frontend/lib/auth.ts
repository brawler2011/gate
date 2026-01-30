"use server";

import { cache } from "react";
import { Call } from "./api";

/**
 * User data from GetMe API
 */
export type SessionUser = {
  id: string;
  username: string;
  role: "admin" | "user";
} | null;

/**
 * Get current authenticated user from backend GetMe API
 * Wrapped in React cache() to avoid duplicate requests during SSR
 * Returns null if user is not authenticated (403) or any error occurs
 */
export const getCurrentUser = cache(async (): Promise<SessionUser> => {
  const [error, response] = await Call((client) => client.default.getMe());

  if (error || !response) {
    // 401 means not authenticated - this is expected, don't log
    if (error && error.status !== 401) {
      console.log("GetMe error:", error);
    }
    return null;
  }

  console.log("GetMe response:", response);

  if (!response.user) {
    console.log("GetMe: no user in response");
    return null;
  }

  const { user } = response;
  const role = user.role === "admin" ? "admin" : "user";

  return {
    id: user.id,
    username: user.username,
    role,
  };
});

/**
 * Check if user is authenticated
 * Returns true if GetMe returns user data, false otherwise
 */
export async function isAuthenticated(): Promise<boolean> {
  const user = await getCurrentUser();
  return user !== null;
}
