"use client";

import { Button } from "@mantine/core";

export function RefreshButton() {
  return (
    <Button onClick={() => window.location.reload()} variant="filled">
      Обновить страницу
    </Button>
  );
}

