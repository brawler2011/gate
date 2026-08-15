"use client";

import {api} from "@/lib/api";
import {
  createWorkshopCheckerFile,
  getWorkshopCheckerFile,
  updateWorkshopCheckerFile,
  setWorkshopCheckerMain,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopCheckersTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="checkers"
      listFiles={(problemId) => api.listProblemCheckers({problemId})}
      getFile={getWorkshopCheckerFile}
      createFile={createWorkshopCheckerFile}
      updateFile={updateWorkshopCheckerFile}
      setMain={setWorkshopCheckerMain}
    />
  );
};
