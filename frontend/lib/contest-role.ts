"use server";

import { Call } from "./api";

/**
 * Contest role types
 * Hierarchy: owner > moderator > participant
 */
export type ContestRole = "owner" | "moderator" | "participant";

export type ContestRoleResponse = {
  role: ContestRole;
  permissionsMask?: number;
} | null;

type ContestRoleApiResponse = {
  role?: unknown;
  permissions_mask?: unknown;
};

const parseContestRoleResponse = (response: unknown): ContestRoleResponse => {
  if (!response || typeof response !== "object") {
    return null;
  }

  const data = response as ContestRoleApiResponse;
  if (data.role !== "owner" && data.role !== "moderator" && data.role !== "participant") {
    return null;
  }

  const parsed: Exclude<ContestRoleResponse, null> = { role: data.role };
  if (typeof data.permissions_mask === "number") {
    parsed.permissionsMask = data.permissions_mask;
  }

  return parsed;
};

/**
 * Get the current user's role in a specific contest
 * 
 * @param contestId - The UUID of the contest
 * @returns The user's role in the contest, or null if not a participant
 */
export async function getMyContestRole(contestId: string): Promise<ContestRoleResponse> {
  const [error, response] = await Call((client) =>
    client.default.getMyContestRole({ contestId })
  );
  if (error || !response) {
    // User is not a participant or not authenticated
    return null;
  }
  
  return parseContestRoleResponse(response);
}
