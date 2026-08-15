"use client";

import {api} from "@/lib/api";
import {
  createWorkshopLibFile,
  deleteWorkshopLibFile,
  getWorkshopLibFile,
  updateWorkshopLibFile,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopLibTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="lib"
      listFiles={(problemId) => api.listProblemLibs({problemId})}
      getFile={getWorkshopLibFile}
      createFile={createWorkshopLibFile}
      updateFile={updateWorkshopLibFile}
      deleteFile={deleteWorkshopLibFile}
    />
  );
};
