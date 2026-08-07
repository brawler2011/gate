import { api } from "@/lib/api";
import { getCurrentUser } from "@/lib/auth";

export async function canManageOrgMembers(orgId: string): Promise<boolean> {
  const currentUser = await getCurrentUser();
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
}
