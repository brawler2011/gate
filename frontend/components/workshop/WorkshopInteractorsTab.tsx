"use client";

import {api} from "@/lib/api";
import {
  createWorkshopInteractorFile,
  getWorkshopInteractorFile,
  updateWorkshopInteractorFile,
} from "@/lib/workshop";

import {WorkshopSingleComponentTab} from "./WorkshopSingleComponentTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopInteractorsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopSingleComponentTab
      {...props}
      componentType="interactor"
      componentTitle="Интерактор"
      defaultFileName="interactor"
      listFiles={(problemId) => api.listProblemInteractors({problemId})}
      getFile={getWorkshopInteractorFile}
      createFile={createWorkshopInteractorFile}
      updateFile={updateWorkshopInteractorFile}
      deleteFile={(problemId, name) => api.deleteProblemInteractor({problemId, name})}
    />
  );
};
