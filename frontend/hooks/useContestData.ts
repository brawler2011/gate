import {useQueryClient, type QueryKey} from "@tanstack/react-query";
import {useEffect} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {GetContestResponseModel, UserModel} from "@/contracts/core/v1/schema";

export const contestQueryKey = (orgLogin: string, contestLogin: string): QueryKey =>
  api.queryOptions("get", "/organizations/{org_login}/contests/{contest_login}", {
    params: {path: {org_login: orgLogin, contest_login: contestLogin}},
  }).queryKey;

export const useContestData = (
  slug: string,
  contestLogin: string,
): {
  error: unknown;
  data?: { response: GetContestResponseModel; user: UserModel | null };
} => {
  const {user, isLoading} = useSession();
  const client = useQueryClient();
  const contest = api.useQuery(
    "get",
    "/organizations/{org_login}/contests/{contest_login}",
    {
      params: {
        path: {org_login: slug, contest_login: contestLogin},
      },
    },
    {enabled: !isLoading},
  );
  const startTime = contest.data?.contest.start_time;
  const endTime = contest.data?.contest.end_time;
  useEffect(() => {
    const next = [startTime, endTime].filter((value): value is string => !!value)
      .map((value) => Date.parse(value)).filter((value) => value > Date.now()).sort((a, b) => a - b)[0];
    if (!next) {
      return;
    }
    // Browser timers cannot represent delays beyond a signed 32-bit interval.
    const timer = setTimeout(() => {
      void client.invalidateQueries({queryKey: contestQueryKey(slug, contestLogin)});
    }, Math.min(next - Date.now() + 1, 2147483647));
    return () => clearTimeout(timer);
  }, [client, startTime, endTime, contest.dataUpdatedAt, slug, contestLogin]);
  return {
    error: contest.error,
    data: contest.data && !isLoading ? {response: contest.data, user} : undefined,
  };
};
