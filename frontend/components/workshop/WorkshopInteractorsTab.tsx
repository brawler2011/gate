"use client";

import {
  createWorkshopInteractorFile,
  getWorkshopInteractorFile,
  listWorkshopInteractorFiles,
  updateWorkshopInteractorFile,
  setWorkshopInteractorMain,
} from "@/lib/actions";
import { WorkshopCollectionTab } from "./WorkshopCollectionTab";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";

export function WorkshopInteractorsTab(props: WorkshopFileTabProps) {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="interactors"
      listFiles={listWorkshopInteractorFiles}
      getFile={getWorkshopInteractorFile}
      createFile={createWorkshopInteractorFile}
      updateFile={updateWorkshopInteractorFile}
      setMain={setWorkshopInteractorMain}
    />
  );
}
