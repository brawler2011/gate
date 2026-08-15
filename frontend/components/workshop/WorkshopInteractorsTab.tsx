"use client";

import {api} from "@/lib/api";
import {
  createWorkshopInteractorFile,
  getWorkshopInteractorFile,
  updateWorkshopInteractorFile,
  setWorkshopInteractorMain,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopInteractorsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="interactors"
      listFiles={(problemId) => api.listProblemInteractors({problemId})}
      getFile={getWorkshopInteractorFile}
      createFile={createWorkshopInteractorFile}
      updateFile={updateWorkshopInteractorFile}
      setMain={setWorkshopInteractorMain}
    />
  );
};
