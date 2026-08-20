"use client";

import {api} from "@/lib/api";

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
      getFile={(problemId, name) => api.getProblemChecker({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemChecker({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemChecker({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemChecker({problemId, name})}
    />
  );
};
