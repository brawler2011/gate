"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  type ReactNode,
} from "react";
import useSWR from "swr";

import {api} from "@/lib/api";

import type {UserModel} from "@/contracts/core/v1";

export type SessionContextType = {
  user: UserModel | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
  setUser: (user: UserModel | null) => void;
  refetchUser: () => Promise<void>;
};

const SessionContext = createContext<SessionContextType | undefined>(undefined);

const fetchSessionUser = async (): Promise<UserModel | null> => {
  try {
    const [, res] = await api.getMe();
    return res?.user ?? null;
  } catch {
    return null;
  }
};

export type SessionProviderProps = {
  children: ReactNode;
  initialUser?: UserModel | null;
};

export const SessionProvider = ({
  children,
  initialUser,
}: SessionProviderProps): ReactNode => {
  const {data, mutate, isLoading} = useSWR<UserModel | null>(
    "current_session_user",
    fetchSessionUser,
    {
      fallbackData: initialUser ?? undefined,
      revalidateOnFocus: true,
      shouldRetryOnError: false,
      dedupingInterval: 5000,
    },
  );

  const user = data ?? null;

  const setUser = useCallback(
    (nextUser: UserModel | null) => {
      void mutate(nextUser, false);
    },
    [mutate],
  );

  const refetchUser = useCallback(async () => {
    await mutate();
  }, [mutate]);

  const value = useMemo<SessionContextType>(
    () => ({
      user,
      isLoading,
      isAuthenticated: Boolean(user),
      isAdmin: user?.role === "admin",
      setUser,
      refetchUser,
    }),
    [user, isLoading, setUser, refetchUser],
  );

  return (
    <SessionContext.Provider value={value}>
      {children}
    </SessionContext.Provider>
  );
};

export const useSession = (): SessionContextType => {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used within a SessionProvider");
  }
  return context;
};
