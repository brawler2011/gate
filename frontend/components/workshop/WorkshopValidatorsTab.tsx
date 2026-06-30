"use client";

import {
  createWorkshopValidatorFile,
  getWorkshopValidatorFile,
  listWorkshopValidatorFiles,
  updateWorkshopValidatorFile,
  setWorkshopValidatorMain,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopValidatorsTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="validators"
      listFiles={listWorkshopValidatorFiles}
      getFile={getWorkshopValidatorFile}
      createFile={createWorkshopValidatorFile}
      updateFile={updateWorkshopValidatorFile}
      setMain={setWorkshopValidatorMain}
    />
  );
}
