import {useQueryClient} from "@tanstack/react-query";
import {useCallback, useMemo} from "react";
import {useLocation, useNavigate, useParams as useRouteParams, useSearchParams as useRouteSearchParams} from "react-router-dom";

export {useRouteParams as useParams};
export const usePathname = (): string => useLocation().pathname;
export const useSearchParams = (): URLSearchParams => useRouteSearchParams()[0];
export const useRouter = (): {
  push: (url: string, options?: { scroll?: boolean }) => void;
  replace: (url: string, options?: { scroll?: boolean }) => void;
  back: () => void;
  refresh: () => void;
} => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const refresh = useCallback(() => {
    void queryClient.invalidateQueries();
  }, [queryClient]);
  return useMemo(
    () => ({
      push: (url: string, _options?: { scroll?: boolean }) => navigate(url),
      replace: (url: string, _options?: { scroll?: boolean }) =>
        navigate(url, {replace: true}),
      back: () => navigate(-1),
      refresh,
    }),
    [navigate, refresh],
  );
};
