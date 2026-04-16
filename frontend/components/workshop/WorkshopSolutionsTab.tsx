"use client";

import {
  createWorkshopSolutionFile,
  getWorkshopSolutionFile,
  listWorkshopSolutionFiles,
  updateWorkshopSolutionFile,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopSolutionsTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="solutions"
      listFiles={listWorkshopSolutionFiles}
      getFile={getWorkshopSolutionFile}
      createFile={createWorkshopSolutionFile}
      updateFile={updateWorkshopSolutionFile}
    />
  );
}
