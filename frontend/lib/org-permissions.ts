import { api } from "@/lib/api";

export const canManageOrgMembers = async (orgId: string): Promise<boolean> => {
  const [, me] = await api.getMe();
  const currentUser = me?.user ?? null;
  if (!currentUser) {
    return false;
  }
  if (currentUser.role === "admin") {
    return true;
  }

  const [error, data] = await api.listOrganizationMembers({ id: orgId, page: 1, pageSize: 100 });
  if (error || !data || !data.members) {
    return false;
  }

  const member = data.members.find((m) => m.user_id === currentUser.id);
  return member?.role === "owner" || member?.role === "admin";
};

