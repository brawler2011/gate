"use client";

import { api } from "@/lib/api";
import {
  createWorkshopGeneratorFile,
  getWorkshopGeneratorFile,
  updateWorkshopGeneratorFile,
} from "@/lib/workshop";

import { WorkshopCollectionTab } from "./WorkshopCollectionTab";

import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export const WorkshopGeneratorsTab = (props: WorkshopFileTabProps) => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="generators"
      listFiles={(problemId) => api.listProblemGenerators({ problemId })}
      getFile={getWorkshopGeneratorFile}
      createFile={createWorkshopGeneratorFile}
      updateFile={updateWorkshopGeneratorFile}
    />
  );
};
