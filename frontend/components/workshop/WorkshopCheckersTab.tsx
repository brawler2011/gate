"use client";

import {api} from "@/lib/api";
import {
  createWorkshopCheckerFile,
  getWorkshopCheckerFile,
  updateWorkshopCheckerFile,
} from "@/lib/workshop";

import {WorkshopSingleComponentTab} from "./WorkshopSingleComponentTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopCheckersTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopSingleComponentTab
      {...props}
      componentType="checker"
      componentTitle="Чекер"
      defaultFileName="checker"
      listFiles={(problemId) => api.listProblemCheckers({problemId})}
      getFile={getWorkshopCheckerFile}
      createFile={createWorkshopCheckerFile}
      updateFile={updateWorkshopCheckerFile}
      deleteFile={(problemId, name) => api.deleteProblemChecker({problemId, name})}
    />
  );
};
