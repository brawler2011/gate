"use client";

import {api} from "@/lib/api";

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
      getFile={(problemId, name) => api.getProblemInteractor({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemInteractor({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemInteractor({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemInteractor({problemId, name})}
    />
  );
};
