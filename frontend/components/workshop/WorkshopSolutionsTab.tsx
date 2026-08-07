"use client";

import { api } from "@/lib/api";
import {
  createWorkshopSolutionFile,
  getWorkshopSolutionFile,
  updateWorkshopSolutionFile,
} from "@/lib/workshop";

import { WorkshopCollectionTab } from "./WorkshopCollectionTab";

import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export const WorkshopSolutionsTab = (props: WorkshopFileTabProps) => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="solutions"
      listFiles={(problemId) => api.listProblemWorkshopSubmissions({ problemId })}
      getFile={getWorkshopSolutionFile}
      createFile={createWorkshopSolutionFile}
      updateFile={updateWorkshopSolutionFile}
    />
  );
};
