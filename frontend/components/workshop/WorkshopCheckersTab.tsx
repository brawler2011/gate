"use client";

import {
  createWorkshopCheckerFile,
  getWorkshopCheckerFile,
  listWorkshopCheckerFiles,
  updateWorkshopCheckerFile,
  setWorkshopCheckerMain,
} from "@/lib/actions";

import { WorkshopCollectionTab } from "./WorkshopCollectionTab";

import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export const WorkshopCheckersTab = (props: WorkshopFileTabProps) => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="checkers"
      listFiles={listWorkshopCheckerFiles}
      getFile={getWorkshopCheckerFile}
      createFile={createWorkshopCheckerFile}
      updateFile={updateWorkshopCheckerFile}
      setMain={setWorkshopCheckerMain}
    />
  );
};
