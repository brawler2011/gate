import { getOrganization } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import { Header } from "./Header";

type HeaderWithSessionProps = {
  secondaryNavItems?: HeaderSecondaryNavItem[];
  organizationId?: string;
};

export async function HeaderWithSession({
  secondaryNavItems,
  organizationId,
}: HeaderWithSessionProps = {}) {
  const [user, organizationResult] = await Promise.all([
    getCurrentUser(),
    organizationId
      ? getOrganization(organizationId)
      : Promise.resolve([null, null] as const),
  ]);

  const organization = organizationResult[1]?.organization
    ? {
        id: organizationResult[1].organization.id,
        name: organizationResult[1].organization.name,
      }
    : undefined;

  return (
    <Header
      user={user}
      secondaryNavItems={secondaryNavItems}
      organization={organization}
    />
  );
}
