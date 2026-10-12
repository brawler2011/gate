"use client";

import {MantineProvider} from "@mantine/core";
import {Notifications} from "@mantine/notifications";

import {SessionProvider} from "@/contexts/SessionContext";
import {theme} from "@/lib/theme/theme";

import type {ReactNode} from "react";

export const Providers = ({
  children,
}: {
  children: ReactNode;
}): ReactNode => {
  return (
    <SessionProvider>
      <MantineProvider
        theme={theme}
        defaultColorScheme="dark"
        withGlobalClasses
      >
        <Notifications />
        {children}
      </MantineProvider>
    </SessionProvider>
  );
};
