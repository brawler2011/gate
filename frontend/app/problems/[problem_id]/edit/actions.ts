"use server";

import { updateProblem as updateProblemAction } from "@/lib/actions";
import * as corev1 from "@contracts/core/v1";

export const UpdateProblem = async (
  id: string,
  data: {
    title?: string;
    time_limit?: number;
    memory_limit?: number;
    legend?: string;
    input_format?: string;
    output_format?: string;
    notes?: string;
    scoring?: string;
  }
) => {
  // Build request object, only including defined and non-empty fields
  const request: corev1.UpdateProblemRequestModel = {};
  
  if (data.title !== undefined && data.title !== "") request.title = data.title;
  if (data.time_limit !== undefined) request.time_limit = data.time_limit;
  if (data.memory_limit !== undefined) request.memory_limit = data.memory_limit;
  if (data.legend !== undefined && data.legend !== "") request.legend = data.legend;
  if (data.input_format !== undefined && data.input_format !== "") request.input_format = data.input_format;
  if (data.output_format !== undefined && data.output_format !== "") request.output_format = data.output_format;
  if (data.notes !== undefined && data.notes !== "") request.notes = data.notes;
  if (data.scoring !== undefined && data.scoring !== "") request.scoring = data.scoring;

  const [error, response] = await updateProblemAction(id, request);
  
  if (error) {
    console.error("❌ UpdateProblem - failed:", error);
    return [error, null] as const;
  }
  
  console.log("✅ UpdateProblem - success");
  return [null, response] as const;
};
