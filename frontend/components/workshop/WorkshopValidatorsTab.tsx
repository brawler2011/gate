"use client";

import {api} from "@/lib/api";

import {WorkshopSingleComponentTab} from "./WorkshopSingleComponentTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopValidatorsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopSingleComponentTab
      {...props}
      componentType="validator"
      componentTitle="Валидатор"
      defaultFileName="validator"
      listFiles={(problemId) => api.listProblemValidators({problemId})}
      getFile={(problemId, name) => api.getProblemValidator({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemValidator({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemValidator({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemValidator({problemId, name})}
    />
  );
};
