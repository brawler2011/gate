"use client";

import {Button} from "@mantine/core";

import type {ReactNode} from "react";

export const RefreshButton = (): ReactNode => {
  return (
    <Button onClick={() => window.location.reload()} variant="filled">
      Обновить страницу
    </Button>
  );
};
