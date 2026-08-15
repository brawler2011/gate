"use client";

import {WorkshopTestsManager} from "./tests/WorkshopTestsManager";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopTestsTab = (props: WorkshopFileTabProps): ReactNode => {
  return <WorkshopTestsManager problemId={props.problemId} />;
};
