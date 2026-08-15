"use client";

import {api} from "@/lib/api";
import {
  createWorkshopGeneratorFile,
  getWorkshopGeneratorFile,
  updateWorkshopGeneratorFile,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopGeneratorsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="generators"
      listFiles={(problemId) => api.listProblemGenerators({problemId})}
      getFile={getWorkshopGeneratorFile}
      createFile={createWorkshopGeneratorFile}
      updateFile={updateWorkshopGeneratorFile}
    />
  );
};
