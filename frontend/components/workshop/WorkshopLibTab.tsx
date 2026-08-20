"use client";

import {api} from "@/lib/api";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopLibTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="lib"
      listFiles={(problemId) => api.listProblemLibs({problemId})}
      getFile={(problemId, name) => api.getProblemLib({problemId, name})}
      createFile={(problemId, name, requestBody) => api.createProblemLib({problemId, name, requestBody})}
      updateFile={(problemId, name, requestBody) => api.updateProblemLib({problemId, name, requestBody})}
      deleteFile={(problemId, name) => api.deleteProblemLib({problemId, name})}
    />
  );
};
