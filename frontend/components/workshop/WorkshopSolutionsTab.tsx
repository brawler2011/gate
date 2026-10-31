"use client";

import {api} from "@/lib/api";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopSolutionsTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="solutions"
      listFiles={(problemId) =>
        api.GET("/problems/{problemId}/submissions", {
          params: {path: {problemId}},
        })
      }
      getFile={(problemId, name) =>
        api.GET("/problems/{problemId}/submissions/{name}", {
          params: {path: {problemId, name}},
          parseAs: "text",
        })
      }
      createFile={(problemId, name, requestBody) =>
        api.POST("/problems/{problemId}/submissions", {
          params: {path: {problemId}, query: {name}},
          body: requestBody,
        })
      }
      updateFile={(problemId, name, requestBody) =>
        api.PUT("/problems/{problemId}/submissions/{name}", {
          params: {path: {problemId, name}},
          body: requestBody,
        })
      }
      deleteFile={(problemId, name) =>
        api.DELETE("/problems/{problemId}/submissions/{name}", {
          params: {path: {problemId, name}},
        })
      }
    />
  );
};
