import {useQuery} from "@tanstack/react-query";
import {Outlet, useParams} from "react-router-dom";

import {OrgHeaderNav} from "@/components/orgs";
import {DefaultLayout} from "@/components/shared";
import {useSession} from "@/contexts/SessionContext";
import {useGetOrganization} from "@/contracts/core/v1/api";
import {PageQueryState} from "@/lib/pageQuery";
import {canManageOrgMembers} from "@/lib/permissions";

import type {ReactNode} from "react";

const OrgLayout = (): ReactNode => {
  const {slug} = useParams() as { slug: string };
  const {user} = useSession();
  const organization = useGetOrganization(slug);
  const membership = useQuery({
    queryKey: ["org-members-permission", slug, user?.id],
    queryFn: () => canManageOrgMembers(slug),
    enabled: Boolean(user),
  });
  if (organization.error) {
    return <Outlet />;
  }
  if (!organization.data) {
    return <PageQueryState query={organization} />;
  }
  const org = organization.data.organization;
  return (
    <DefaultLayout
      headerUser={user}
      headerOrganization={{id: org.id, login: org.login, name: org.name}}
      headerSecondaryNav={<OrgHeaderNav orgLogin={org.login} showMembersTab={membership.data ?? false} />}
    >
      <Outlet />
    </DefaultLayout>
  );
};
export default OrgLayout;
