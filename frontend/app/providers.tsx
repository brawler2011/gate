"use client";

import {MantineProvider} from "@mantine/core";
import {Notifications} from "@mantine/notifications";

import {theme} from "@/lib/theme/theme";

import type {ReactNode} from "react";

export const Providers = ({children}: { children: ReactNode }): ReactNode => {
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
};
