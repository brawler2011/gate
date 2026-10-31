import {useSession} from "@/contexts/SessionContext";
import {useGetContest, useGetMyContestRole} from "@/contracts/core/v1/api";
import {parseContestRoleResponse} from "@/lib/permissions";

import type {GetContestResponseModel, UserModel} from "@/contracts/core/v1/models";
import type {ContestRoleResponse} from "@/lib/permissions";

export const useContestData = (
  slug: string,
  contestLogin: string,
): {
  error: unknown;
  data?: { response: GetContestResponseModel; user: UserModel | null; contestRole: ContestRoleResponse };
} => {
  const {user, isLoading} = useSession();
  const contest = useGetContest(slug, contestLogin, {
    query: {enabled: !isLoading},
  });
  const role = useGetMyContestRole(slug, contestLogin, {
    query: {enabled: Boolean(user)},
  });
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
