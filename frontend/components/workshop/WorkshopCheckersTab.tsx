"use client";

import {api} from "@/lib/api";

import {WorkshopSingleComponentTab} from "./WorkshopSingleComponentTab";

import type {WorkshopFileTabProps} from "./WorkshopFileTabProps";
import type {ReactNode} from "react";

export const WorkshopCheckersTab = (props: WorkshopFileTabProps): ReactNode => {
  return (
    <WorkshopSingleComponentTab
      {...props}
      componentType="checker"
      componentTitle="Чекер"
      defaultFileName="checker"
      listFiles={(problemId) =>
        api.GET("/problems/{problemId}/checkers", {
          params: {path: {problemId}},
        })
      }
      getFile={(problemId, name) =>
        api.GET("/problems/{problemId}/checkers/{name}", {
          params: {path: {problemId, name}},
          parseAs: "text",
        })
      }
      createFile={(problemId, name, requestBody) =>
        api.POST("/problems/{problemId}/checkers", {
          params: {path: {problemId}, query: {name}},
          body: requestBody,
        })
      }
      updateFile={(problemId, name, requestBody) =>
        api.PUT("/problems/{problemId}/checkers/{name}", {
          params: {path: {problemId, name}},
          body: requestBody,
        })
      }
      deleteFile={(problemId, name) =>
        api.DELETE("/problems/{problemId}/checkers/{name}", {
          params: {path: {problemId, name}},
        })
      }
    />
  );
};
