import {Stack} from "@mantine/core";
import {Outlet} from "react-router-dom";

import type {ReactNode} from "react";

const AuthLayout = (): ReactNode => {
  return (
    <main
      style={{
        padding: "var(--mantine-spacing-md)",
        paddingBottom: "var(--mantine-spacing-xl)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexDirection: "column",
        gap: "var(--mantine-spacing-xl)",
        minHeight: "100vh",
        backgroundColor: "var(--mantine-color-body)",
      }}
    >
      <Stack gap="xl" align="center">
        <Outlet />
      </Stack>
    </main>
  );
};

export default AuthLayout;
