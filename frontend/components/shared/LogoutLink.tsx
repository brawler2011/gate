import {Button} from "@mantine/core";
import {useState} from "react";
import {useNavigate} from "react-router-dom";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {ButtonProps} from "@mantine/core";
import type {ReactNode} from "react";

type LogoutLinkProps = ButtonProps & { children?: ReactNode };

const LogoutLink = (props: LogoutLinkProps): ReactNode => {
  const navigate = useNavigate();
  const {setUser} = useSession();
  const [loading, setLoading] = useState(false);

  const handleLogout = async () => {
    setLoading(true);
    try {
      await api.POST("/auth/logout");
      setUser(null);
      navigate("/auth/login");
    } catch {
      setUser(null);
      navigate("/auth/login");
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
