import { api } from "@/lib/api";
import { getCurrentUser } from "@/lib/auth";

import {
  Header,
  type HeaderContest,
  type HeaderOrganization,
  type HeaderProblem,
} from "./Header";

import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";

type HeaderWithSessionProps = {
  secondaryNavItems?: HeaderSecondaryNavItem[];
  organizationId?: string;
  organization?: HeaderOrganization;
  contest?: HeaderContest;
  problem?: HeaderProblem;
};

export const HeaderWithSession = async ({
  secondaryNavItems,
  organizationId,
  organization: passedOrganization,
  contest,
  problem,
}: HeaderWithSessionProps = {}) => {
  const targetOrgId = passedOrganization?.id || organizationId;

  const [user, organizationResult] = await Promise.all([
    getCurrentUser(),
    !passedOrganization && targetOrgId
      ? api.getOrganization({ id: targetOrgId })
      : Promise.resolve([null, null] as const),
  ]);

  const getOrganizationObj = () => {
    if (passedOrganization) {
      return passedOrganization;
    }
    const org = organizationResult[1]?.organization;
    if (org) {
      return { id: org.id, name: org.name };
    }
    return undefined;
  };

  const organization = getOrganizationObj();

  return (
    <Header
      user={user}
      secondaryNavItems={secondaryNavItems}
      organization={organization}
      contest={contest}
      problem={problem}
    />
  );
};
