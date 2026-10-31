"use client";

import {api} from "@/lib/api";

import {WorkshopCollectionTab} from "./WorkshopCollectionTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopLibTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopCollectionTab
      {...props}
      folderName="lib"
      listFiles={(problemId) =>
        api.GET("/problems/{problemId}/lib", {
          params: {path: {problemId}},
        })
      }
      getFile={(problemId, name) =>
        api.GET("/problems/{problemId}/lib/{name}", {
          params: {path: {problemId, name}},
          parseAs: "text",
        })
      }
      createFile={(problemId, name, requestBody) =>
        api.POST("/problems/{problemId}/lib", {
          params: {path: {problemId}, query: {name}},
          body: requestBody,
        })
      }
      updateFile={(problemId, name, requestBody) =>
        api.PUT("/problems/{problemId}/lib/{name}", {
          params: {path: {problemId, name}},
          body: requestBody,
        })
      }
      deleteFile={(problemId, name) =>
        api.DELETE("/problems/{problemId}/lib/{name}", {
          params: {path: {problemId, name}},
        })
      }
    />
  );
};
