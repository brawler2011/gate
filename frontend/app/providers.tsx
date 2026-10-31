"use client";

import {MantineProvider} from "@mantine/core";
import {Notifications} from "@mantine/notifications";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";

import {SessionProvider} from "@/contexts/SessionContext";
import {theme} from "@/lib/theme/theme";

import type {ReactNode} from "react";

const queryClient = new QueryClient({defaultOptions: {queries: {retry: false, staleTime: 10000}}});

export const Providers = ({
  children,
}: {
  children: ReactNode;
}): ReactNode => {
  return (
    <QueryClientProvider client={queryClient}>
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
    </QueryClientProvider>
  );
};
