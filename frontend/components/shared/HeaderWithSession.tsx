import { api } from "@/lib/api";

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

  const [meResult, organizationResult] = await Promise.all([
    api.getMe(),
    !passedOrganization && targetOrgId
      ? api.getOrganization({ id: targetOrgId })
      : Promise.resolve([null, null] as const),
  ]);

  const user = meResult[1]?.user ?? null;

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
