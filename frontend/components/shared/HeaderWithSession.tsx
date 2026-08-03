import { getOrganization } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";
import {
  Header,
  type HeaderContest,
  type HeaderOrganization,
  type HeaderProblem,
} from "./Header";

type HeaderWithSessionProps = {
  secondaryNavItems?: HeaderSecondaryNavItem[];
  organizationId?: string;
  organization?: HeaderOrganization;
  contest?: HeaderContest;
  problem?: HeaderProblem;
};

export async function HeaderWithSession({
  secondaryNavItems,
  organizationId,
  organization: passedOrganization,
  contest,
  problem,
}: HeaderWithSessionProps = {}) {
  const targetOrgId = passedOrganization?.id || organizationId;

  const [user, organizationResult] = await Promise.all([
    getCurrentUser(),
    !passedOrganization && targetOrgId
      ? getOrganization(targetOrgId)
      : Promise.resolve([null, null] as const),
  ]);

  const organization = passedOrganization
    ? passedOrganization
    : organizationResult[1]?.organization
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
      contest={contest}
      problem={problem}
    />
  );
}
