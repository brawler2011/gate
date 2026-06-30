"use client";

import {
  createWorkshopGeneratorFile,
  getWorkshopGeneratorFile,
  listWorkshopGeneratorFiles,
  updateWorkshopGeneratorFile,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopGeneratorsTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="generators"
      listFiles={listWorkshopGeneratorFiles}
      getFile={getWorkshopGeneratorFile}
      createFile={createWorkshopGeneratorFile}
      updateFile={updateWorkshopGeneratorFile}
    />
  );
}
