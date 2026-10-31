import {Center, Loader} from "@mantine/core";
import {useQuery} from "@tanstack/react-query";
import {Navigate} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {useSession} from "@/contexts/SessionContext";
import {RouteRedirect} from "@/lib/routeErrors";

import type {UseQueryResult, QueryKey} from "@tanstack/react-query";
import type {ReactNode} from "react";

export const usePageQuery = <T,>(
  key: QueryKey,
  load: () => Promise<T>,
): UseQueryResult<T> => {
  const {user, isLoading} = useSession();
  return useQuery({
    queryKey: [...key, user?.id ?? null],
    queryFn: load,
    enabled: !isLoading,
    retry: false,
  });
};
export const PageQueryState = ({
  query,
}: {
  query: {error: unknown};
}): ReactNode => {
  if (query.error instanceof RouteRedirect) {
    return <Navigate to={query.error.to} replace />;
  }
  if (query.error) {
    const error = query.error as { status?: number; message?: string };
    return (
      <ErrorDisplay
        error={{
          status: error.status ?? 500,
          message: error.message ?? "Ошибка загрузки",
        }}
      />
    );
  }
  return (
    <Center py="xl">
      <Loader />
    </Center>
  );
};
