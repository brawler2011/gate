import {isRouteErrorResponse, useRouteError} from "react-router-dom";

import NotFound from "@/app/not-found";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";

import type {ReactNode} from "react";

export const RouteError = (): ReactNode => {
  const error = useRouteError();
  if (isRouteErrorResponse(error)) {
    if (error.status === 404) {
      return <NotFound />;
    }
    return <ErrorDisplay error={{status: error.status, message: error.statusText}} />;
  }
  const failure = error as {status?: number; message?: string};
  if (failure?.status === 404) {
    return <NotFound />;
  }
  return <ErrorDisplay error={{status: failure?.status ?? 500, message: failure?.message ?? "Ошибка загрузки"}} />;
};
