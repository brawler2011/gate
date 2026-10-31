import {Center, Loader} from "@mantine/core";

import {ErrorDisplay} from "./ErrorDisplay";

import type {ReactNode} from "react";

export const QueryState = ({
  error,
}: {
  error?: unknown;
}): ReactNode => {
  if (error) {
    const failure = error as { status?: number; message?: string; info?: {message?: string} };
    return (
      <ErrorDisplay
        error={{
          status: failure.status ?? 500,
          message: failure.info?.message ?? failure.message ?? "Ошибка загрузки",
        }}
      />
    );
  }
  return (
    <Center py="xl">
      <Loader />
    </Center>
  );
};
