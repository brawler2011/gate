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
