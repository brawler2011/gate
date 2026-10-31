import {MantineProvider} from "@mantine/core";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {render, type RenderOptions, type RenderResult} from "@testing-library/react";
import React, {type ReactElement, type ReactNode} from "react";
import {MemoryRouter} from "react-router-dom";

import {RouteBoundary} from "@/components/shared/RouteBoundary";
import {SessionContext} from "@/contexts/SessionContext";

import {setupDOMEnvironment} from "./setup-dom";

export {setupDOMEnvironment};

const AllTheProviders = ({children, route}: { children: ReactNode; route?: string }) => {
  const [client] = React.useState(
    () =>
      new QueryClient({
        defaultOptions: {queries: {retry: false, gcTime: 0}},
      }),
  );
  return (
    <MemoryRouter initialEntries={route ? [route] : undefined}>
      <QueryClientProvider client={client}>
        <SessionContext.Provider
          value={{
            user: null,
            isLoading: false,
            isAuthenticated: false,
            isAdmin: false,
            setUser: () => {},
            refetchUser: async () => {},
          }}
        >
          <MantineProvider>
            <RouteBoundary>{children}</RouteBoundary>
          </MantineProvider>
        </SessionContext.Provider>
      </QueryClientProvider>
    </MemoryRouter>
  );
};

export const renderWithProviders = (
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper"> & {route?: string},
): RenderResult => {
  const {route, ...renderOptions} = options ?? {};
  return render(ui, {wrapper: ({children}) => <AllTheProviders route={route}>{children}</AllTheProviders>,
    ...renderOptions});
};

export * from "@testing-library/react";
