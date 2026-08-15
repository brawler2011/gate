"use client";

import {api} from "@/lib/api";
import {
  createWorkshopSolutionFile,
  getWorkshopSolutionFile,
  updateWorkshopSolutionFile,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopSolutionsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="solutions"
      listFiles={(problemId) => api.listProblemWorkshopSubmissions({problemId})}
      getFile={getWorkshopSolutionFile}
      createFile={createWorkshopSolutionFile}
      updateFile={updateWorkshopSolutionFile}
    />
  );
};
