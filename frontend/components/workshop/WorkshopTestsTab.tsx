"use client";

import {
  createWorkshopTestFile,
  getWorkshopTestFile,
  listWorkshopTestFiles,
  updateWorkshopTestFile,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopTestsTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="tests"
      listFiles={listWorkshopTestFiles}
      getFile={getWorkshopTestFile}
      createFile={createWorkshopTestFile}
      updateFile={updateWorkshopTestFile}
    />
  );
}
