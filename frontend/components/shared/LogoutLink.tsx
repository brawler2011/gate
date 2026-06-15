"use client";

import type { ButtonProps } from "@mantine/core";
import { Button } from "@mantine/core";
import type { ReactNode } from "react";
import { useState } from "react";
import { logoutAction } from "@lib/auth-actions";

type LogoutLinkProps = ButtonProps & { children?: ReactNode };

const LogoutLink = (props: LogoutLinkProps) => {
  const [loading, setLoading] = useState(false);

  const handleLogout = async () => {
    setLoading(true);
    try {
      await logoutAction();
      window.location.href = "/auth/login";
    } catch {
      window.location.href = "/auth/login";
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button onClick={handleLogout} loading={loading} {...props}>
      {props.children ?? "Выйти"}
    </Button>
  );
};

export { LogoutLink };
