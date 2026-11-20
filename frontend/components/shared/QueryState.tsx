import {Center, Loader} from "@mantine/core";

import {ErrorDisplay} from "./ErrorDisplay";

import type {ReactNode} from "react";

export const QueryState = ({
  error,
}: {
  error?: unknown;
}): ReactNode => {
  if (error) {
    const failure = error as {status?: number; request_id?: string};
    return <ErrorDisplay error={{status: failure.status ?? 500, request_id: failure.request_id}} />;
  }
  return (
    <Center py="xl">
      <Loader />
    </Center>
  );
};
