"use client";

import {
  createWorkshopMediaFile,
  getWorkshopMediaFile,
  listWorkshopMediaFiles,
  updateWorkshopMediaFile,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopMediaTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="media"
      listFiles={listWorkshopMediaFiles}
      getFile={getWorkshopMediaFile}
      createFile={createWorkshopMediaFile}
      updateFile={updateWorkshopMediaFile}
    />
  );
}
