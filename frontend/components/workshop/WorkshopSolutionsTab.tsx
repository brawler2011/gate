"use client";

import {api} from "@/lib/api";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopSolutionsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="solutions"
      listFiles={(problemId) => api.listProblemWorkshopSubmissions({problemId})}
      getFile={(problemId, name) => api.getProblemWorkshopSubmission({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemWorkshopSubmission({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemWorkshopSubmission({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemWorkshopSubmission({problemId, name})}
    />
  );
};
