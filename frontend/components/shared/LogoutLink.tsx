"use client";

import {Button} from "@mantine/core";
import {useState} from "react";

import {api} from "@/lib/api";

import type {ButtonProps} from "@mantine/core";
import type {ReactNode} from "react";

type LogoutLinkProps = ButtonProps & { children?: ReactNode };

const LogoutLink = (props: LogoutLinkProps): ReactNode => {
  const [loading, setLoading] = useState(false);

  const handleLogout = async () => {
    setLoading(true);
    try {
      await api.logout();
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

export {LogoutLink};
