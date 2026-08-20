"use client";

import {api} from "@/lib/api";

import {WorkshopSingleComponentTab} from "./WorkshopSingleComponentTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopGeneratorsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopSingleComponentTab
      {...props}
      componentType="generator"
      componentTitle="Генератор"
      defaultFileName="generator"
      listFiles={(problemId) => api.listProblemGenerators({problemId})}
      getFile={(problemId, name) => api.getProblemGenerator({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemGenerator({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemGenerator({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemGenerator({problemId, name})}
    />
  );
};
