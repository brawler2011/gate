"use client";

import {api} from "@/lib/api";
import {
  createWorkshopValidatorFile,
  getWorkshopValidatorFile,
  updateWorkshopValidatorFile,
} from "@/lib/workshop";

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
      getFile={getWorkshopValidatorFile}
      createFile={createWorkshopValidatorFile}
      updateFile={updateWorkshopValidatorFile}
      deleteFile={(problemId, name) => api.deleteProblemValidator({problemId, name})}
    />
  );
};
