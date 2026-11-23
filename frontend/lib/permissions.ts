import type {GetMyContestRoleResponseModel, OrganizationPermissionsModel} from "@/contracts/core/v1/schema";

export type ContestRoleResponse = GetMyContestRoleResponseModel | null;
export const parseContestRoleResponse = (response: unknown): ContestRoleResponse => {
  if (!response || typeof response !== "object") {
    return null;
  }
  const data = response as Record<string, unknown>;
  if (typeof data.role !== "string") {
    return null;
  }
  return {role: data.role};
};

export const getMyContestRole = async (orgLogin: string, contestLogin: string): Promise<ContestRoleResponse> => {
  const {api} = await import("@/lib/api");
  const {data: response, error} = await api.GET(
    "/organizations/{org_login}/contests/{contest_login}/my-role",
    {
      params: {
        path: {org_login: orgLogin, contest_login: contestLogin},
      },
    },
  );
  if (error || !response) {
    return null;
  }

  return parseContestRoleResponse(response);
};

export const getOrgPermissions = async (orgLogin: string): Promise<OrganizationPermissionsModel | null> => {
  const {api} = await import("@/lib/api");
  const {data, error} = await api.GET("/organizations/{login}", {params: {path: {login: orgLogin}}});
  return error ? null : data?.organization.permissions ?? null;
};

// Maintainers manage invitations; member management needs an administrator.
export const canOpenOrgMembers = (p: OrganizationPermissionsModel | null): boolean =>
  !!p && (p.invite || p.manage_members);
