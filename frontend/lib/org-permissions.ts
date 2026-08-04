import { listOrganizationMembers } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";

export async function canManageOrgMembers(orgId: string): Promise<boolean> {
  const currentUser = await getCurrentUser();
  if (!currentUser) return false;
  if (currentUser.role === "admin") return true;

  const [error, data] = await listOrganizationMembers(orgId, 1, 100);
  if (error || !data) return false;

  const member = data.members.find((m) => m.user_id === currentUser.id);
  return member?.role === "owner" || member?.role === "admin";
}
