import {api} from "@/lib/api";

export const canManageOrgMembers = async (orgId: string): Promise<boolean> => {
  const [, me] = await api.getMe();
  const currentUser = me?.user ?? null;
  if (!currentUser) {
    return false;
  }
  if (currentUser.role === "admin") {
    return true;
  }

  let page = 1;
  while (page <= 10) {
    const [error, data] = await api.listOrganizationMembers({id: orgId, page, pageSize: 100});
    if (error || !data || !data.members) {
      return false;
    }

    const member = data.members.find((m) => m.user_id === currentUser.id);
    if (member) {
      return member.role === "owner" || member.role === "admin";
    }

    const total = data.pagination?.total ?? 0;
    const totalPages = Math.ceil(total / 100);
    if (page >= totalPages || totalPages === 0) {
      break;
    }
    page++;
  }

  return false;
};
