"use client";

import { theme } from "@/lib/theme/theme";
import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import type { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <MantineProvider
      theme={theme}
      defaultColorScheme="dark"
      withGlobalClasses
    >
      <Notifications />
      {children}
    </MantineProvider>
  );
}
