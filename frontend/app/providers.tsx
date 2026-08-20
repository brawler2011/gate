"use client";

import {MantineProvider} from "@mantine/core";
import {Notifications} from "@mantine/notifications";
import {useEffect} from "react";

import {SessionProvider} from "@/contexts/SessionContext";
import {initBrowserTelemetry} from "@/lib/telemetry";
import {theme} from "@/lib/theme/theme";

import type {ReactNode} from "react";

export const Providers = ({
  children,
}: {
  children: ReactNode;
}): ReactNode => {
  useEffect(() => {
    initBrowserTelemetry();
  }, []);

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
