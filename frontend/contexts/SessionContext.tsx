import {useQueryClient} from "@tanstack/react-query";
import {createContext, useCallback, useContext, useMemo} from "react";

import {getGetMeQueryKey, useGetMe} from "@/contracts/core/v1/api";

import type {UserModel} from "@/contracts/core/v1/models";
import type {ReactNode} from "react";

export type SessionContextType = {
  user: UserModel | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
  setUser: (user: UserModel | null) => void;
  refetchUser: () => Promise<void>;
};
export const SessionContext = createContext<SessionContextType | undefined>(
  undefined,
);
export type SessionProviderProps = {
  children: ReactNode;
  initialUser?: UserModel | null;
};
export const SessionProvider = ({
  children,
  initialUser,
}: SessionProviderProps): ReactNode => {
  const queryClient = useQueryClient();
  const {data, isLoading, refetch} = useGetMe({
    query: {
      retry: false,
      staleTime: 5000,
      initialData: initialUser ? {user: initialUser} : undefined,
    },
  });
  const user = data?.user ?? null;
  const setUser = useCallback(
    (nextUser: UserModel | null) => {
      queryClient.clear();
      queryClient.setQueryData(getGetMeQueryKey(), {user: nextUser});
    },
    [queryClient],
  );
  const refetchUser = useCallback(async () => {
    await refetch();
  }, [refetch]);
  const value = useMemo(
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
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
};
export const useSession = (): SessionContextType => {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used within a SessionProvider");
  }
  return context;
};
