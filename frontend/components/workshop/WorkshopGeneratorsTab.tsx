"use client";

import {api} from "@/lib/api";
import {
  createWorkshopGeneratorFile,
  getWorkshopGeneratorFile,
  updateWorkshopGeneratorFile,
} from "@/lib/workshop";

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
      getFile={getWorkshopGeneratorFile}
      createFile={createWorkshopGeneratorFile}
      updateFile={updateWorkshopGeneratorFile}
      deleteFile={(problemId, name) => api.deleteProblemGenerator({problemId, name})}
    />
  );
};
