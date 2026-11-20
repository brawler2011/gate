import {isRouteErrorResponse, useRouteError} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";

export const RouteError = (): ReactNode => {
  const error = useRouteError();
  if (isRouteErrorResponse(error)) {
    if (error.status === 404) {
      return <NotFound />;
    }
    return (
      <>
        <title>Ошибка</title>
        <ErrorDisplay error={{status: error.status}} />
      </>
    );
  }
  const failure = error as {status?: number; request_id?: string};
  if (failure?.status === 404) {
    return <NotFound />;
  }
  return (
    <>
      <title>Ошибка</title>
      <ErrorDisplay error={{status: failure?.status ?? 500, request_id: failure?.request_id}} />
    </>
  );
};
