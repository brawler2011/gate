"use client";

import type { ButtonProps } from "@mantine/core";
import { Button } from "@mantine/core";
import type { ReactNode } from "react";
import { useState } from "react";

type LogoutLinkProps = ButtonProps & { children?: ReactNode };

const LogoutLink = (props: LogoutLinkProps) => {
  const [loading, setLoading] = useState(false);

  const handleLogout = async () => {
    setLoading(true);
    try {
      // Получаем logout URL с токеном от Kratos
      const response = await fetch("/api/.ory/self-service/logout/browser", {
        credentials: "include",
        headers: { Accept: "application/json" },
      });

      if (!response.ok) {
        // Если не авторизован - просто редиректим на login
        window.location.href = "/auth/login";
        return;
      }

      const data = await response.json();

      if (data.logout_token) {
        // Используем прокси с токеном для logout
        window.location.href = `/api/.ory/self-service/logout?token=${data.logout_token}&return_to=/auth/login`;
      } else {
        window.location.href = "/auth/login";
      }
    } catch {
      window.location.href = "/auth/login";
    }
  };

  return (
    <Button onClick={handleLogout} loading={loading} {...props}>
      {props.children ?? "Выйти"}
    </Button>
  );
};

export { LogoutLink };
