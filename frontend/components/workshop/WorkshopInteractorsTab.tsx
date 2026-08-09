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

export const WorkshopInteractorsTab = (props: WorkshopFileTabProps) => {
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
