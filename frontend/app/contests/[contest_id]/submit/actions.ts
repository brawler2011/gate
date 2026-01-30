"use server";

import { createSolution } from "@/lib/actions";
import { LANGUAGE_MAP } from "@/lib/constants";

export async function submitSubmission(
  problemId: string,
  contestId: string,
  submission: FormData,
  language: string
): Promise<number | null> {
  const languageCode = LANGUAGE_MAP[language];
  if (!languageCode) {
    console.error("Invalid language:", language);
    return null;
  }

  const [error, response] = await createSolution(
    problemId,
    contestId,
    languageCode,
    submission
  );

  if (error) {
    console.error("Failed to create submission:", error);
    return null;
  }

  // Return 1 on success (component uses this only to check success, not the actual ID)
  return response?.id ? 1 : null;
}
