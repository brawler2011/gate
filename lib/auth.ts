"use server";

import { cache } from "react";
import { cookies } from "next/headers";
import type { Session } from "@ory/client";

/**
 * User data extracted from Ory Kratos session
 */
export type SessionUser = {
  id: string;
  username: string;
  email: string;
  role: "admin" | "user";
} | null;

/**
 * Get Ory Kratos session for the current user
 * Wrapped in React cache() to avoid duplicate requests during SSR
 */
export const getSession = cache(async (): Promise<Session | null> => {
  try {
    const { FrontendApi, Configuration } = await import("@ory/client");
    const cookieStore = await cookies();

    const ory = new FrontendApi(
      new Configuration({
        basePath: process.env.ORY_SDK_URL,
        baseOptions: {
          withCredentials: true,
        },
      })
    );

    const cookiePairs: string[] = [];
    cookieStore.getAll().forEach((cookie) => {
      cookiePairs.push(`${cookie.name}=${cookie.value}`);
    });

    if (cookiePairs.length > 0) {
      const sessionResponse = await ory.toSession(
        {},
        {
          headers: {
            Cookie: cookiePairs.join("; "),
          },
        }
      );

      return sessionResponse.data;
    }
  } catch {
    return null;
  }
  return null;
});

/**
 * Get current authenticated user from Ory Kratos session
 * Returns null if user is not authenticated
 */
export async function getCurrentUser(): Promise<SessionUser> {
  try {
    const session = await getSession();

    if (!session || !session.identity) {
      return null;
    }

    const { identity } = session;
    const traits = identity.traits as { username?: string; email?: string };
    const metadata = identity.metadata_public as { user_id?: string; role?: string };

    if (!metadata?.user_id || !traits?.username || !traits?.email || !metadata?.role) {
      console.warn("Session is missing required user data");
      return null;
    }

    const role = metadata.role === "admin" ? "admin" : "user";

    return {
      id: metadata.user_id,
      username: traits.username,
      email: traits.email,
      role,
    };
  } catch (error) {
    console.error("Error getting current user:", error);
    return null;
  }
}

