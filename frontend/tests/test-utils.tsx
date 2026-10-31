import {MantineProvider} from "@mantine/core";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {render, type RenderOptions, type RenderResult} from "@testing-library/react";
import React, {type ReactElement, type ReactNode} from "react";
import {createMemoryRouter, RouterProvider} from "react-router-dom";

import {QueryState} from "@/components/shared/QueryState";
import {RouteError} from "@/components/shared/RouteError";
import {SessionContext} from "@/contexts/SessionContext";

import {setupDOMEnvironment} from "./setup-dom";

import type {RouteObject} from "react-router-dom";

export {setupDOMEnvironment};

const AllTheProviders = ({children}: { children: ReactNode }) => {
  const [client] = React.useState(
    () =>
      new QueryClient({
        defaultOptions: {queries: {retry: false, gcTime: 0}},
      }),
  );
  return (
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
          {children}
        </MantineProvider>
      </SessionContext.Provider>
    </QueryClientProvider>
  );
};

export const renderWithProviders = (
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper"> & {route?: string; routes?: RouteObject[]},
): RenderResult & {router: ReturnType<typeof createMemoryRouter>} => {
  const {route = "/", routes, ...renderOptions} = options ?? {};
  const router = createMemoryRouter(routes ?? [{path: "*", element: ui}], {initialEntries: [route]});
  return {...render(<RouterProvider router={router} />, {
    wrapper: AllTheProviders,
    ...renderOptions,
  }), router};
};

export const renderRoute = (
  ui: ReactElement, path: string, loader: RouteObject["loader"], route: string,
): RenderResult => renderWithProviders(ui, {route, routes: [{path, element: ui, loader,
  errorElement: <RouteError />, hydrateFallbackElement: <QueryState />} ]});

export * from "@testing-library/react";
