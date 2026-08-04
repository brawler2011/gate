"use client";

import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";
import { WorkshopTestsManager } from "./tests/WorkshopTestsManager";

export function WorkshopTestsTab(props: WorkshopFileTabProps) {
  return <WorkshopTestsManager problemId={props.problemId} />;
}
