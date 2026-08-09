"use client";

import {api} from "@/lib/api";
import {
  createWorkshopValidatorFile,
  getWorkshopValidatorFile,
  updateWorkshopValidatorFile,
  setWorkshopValidatorMain,
} from "@/lib/workshop";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";

export const WorkshopValidatorsTab = (props: WorkshopFileTabProps) => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="validators"
      listFiles={(problemId) => api.listProblemValidators({problemId})}
      getFile={getWorkshopValidatorFile}
      createFile={createWorkshopValidatorFile}
      updateFile={updateWorkshopValidatorFile}
      setMain={setWorkshopValidatorMain}
    />
  );
};
