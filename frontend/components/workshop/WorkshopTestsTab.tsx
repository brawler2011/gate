"use client";

import { WorkshopTestsManager } from "./tests/WorkshopTestsManager";

import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export const WorkshopTestsTab = (props: WorkshopFileTabProps) => {
  return <WorkshopTestsManager problemId={props.problemId} />;
};
