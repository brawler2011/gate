"use server";

import { Call } from "./api";

/**
 * Contest role types
 * Hierarchy: owner > moderator > participant
 */
export type ContestRole = "owner" | "moderator" | "participant";

export type ContestRoleResponse = {
  role: ContestRole;
} | null;

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
  
  // Validate that the role is one of the expected values
  const role = response.role as ContestRole;
  if (role === "owner" || role === "moderator" || role === "participant") {
    return { role };
  }
  
  // If role is not recognized, treat as no role
  return null;
}
