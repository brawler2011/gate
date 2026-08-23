import {MantineProvider} from "@mantine/core";
import {render, type RenderOptions, type RenderResult} from "@testing-library/react";
import React, {type ReactElement, type ReactNode} from "react";

import {setupDOMEnvironment} from "./setup-dom";

export {setupDOMEnvironment};

const AllTheProviders = ({children}: { children: ReactNode }) => {
  return <MantineProvider>{children}</MantineProvider>;
};

export const renderWithProviders = (
  ui: ReactElement,
  options?: Omit<RenderOptions, "wrapper">
): RenderResult => render(ui, {wrapper: AllTheProviders, ...options});

export * from "@testing-library/react";
