import type {OrganizationPermissionsModel} from "@/contracts/core/v1/schema";

export const getOrgPermissions = async (orgLogin: string): Promise<OrganizationPermissionsModel | null> => {
  const {api} = await import("@/lib/api");
  const {data, error} = await api.GET("/organizations/{login}", {params: {path: {login: orgLogin}}});
  return error ? null : data?.organization.permissions ?? null;
};

// Maintainers manage invitations; member management needs an administrator.
export const canOpenOrgMembers = (p: OrganizationPermissionsModel | null): boolean =>
  !!p && (p.invite || p.manage_members);
