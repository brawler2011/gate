import {useQueryClient} from "@tanstack/react-query";
import {useEffect} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";
import {parseContestRoleResponse} from "@/lib/permissions";

import type {GetContestResponseModel, UserModel} from "@/contracts/core/v1/schema";
import type {ContestRoleResponse} from "@/lib/permissions";

export const useContestData = (
  slug: string,
  contestLogin: string,
): {
  error: unknown;
  data?: { response: GetContestResponseModel; user: UserModel | null; contestRole: ContestRoleResponse };
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
  const role = api.useQuery(
    "get",
    "/organizations/{org_login}/contests/{contest_login}/my-role",
    {
      params: {
        path: {org_login: slug, contest_login: contestLogin},
      },
    },
    {enabled: Boolean(user)},
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
      void client.invalidateQueries({queryKey: api.queryOptions("get", "/organizations/{org_login}/contests/{contest_login}", {params: {path: {org_login: slug, contest_login: contestLogin}}}).queryKey});
      void client.invalidateQueries({queryKey: api.queryOptions("get", "/organizations/{org_login}/contests/{contest_login}/my-role", {params: {path: {org_login: slug, contest_login: contestLogin}}}).queryKey});
    }, Math.min(next - Date.now() + 1, 2147483647));
    return () => clearTimeout(timer);
  }, [client, startTime, endTime, contest.dataUpdatedAt, slug, contestLogin]);
  return {
    error: contest.error,
    data:
      contest.data && !isLoading && !(user && role.isPending)
        ? {
          response: contest.data,
          user,
          contestRole: user ? parseContestRoleResponse(role.data) : null,
        }
        : undefined,
  };
};
