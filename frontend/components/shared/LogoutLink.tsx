"use client";

import {Button} from "@mantine/core";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {ButtonProps} from "@mantine/core";
import type {ReactNode} from "react";

type LogoutLinkProps = ButtonProps & { children?: ReactNode };

const LogoutLink = (props: LogoutLinkProps): ReactNode => {
  const router = useRouter();
  const {setUser} = useSession();
  const [loading, setLoading] = useState(false);

  const handleLogout = async () => {
    setLoading(true);
    try {
      await api.logout();
      setUser(null);
      router.push("/auth/login");
      router.refresh();
    } catch {
      setUser(null);
      router.push("/auth/login");
      router.refresh();
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
