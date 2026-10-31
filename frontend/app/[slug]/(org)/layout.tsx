import {useQuery} from "@tanstack/react-query";
import {Outlet, useParams} from "react-router-dom";

import {OrgHeaderNav} from "@/components/orgs/OrgHeaderNav";
import {DefaultLayout} from "@/components/shared/DefaultLayout";
import {QueryState} from "@/components/shared/QueryState";
import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";
import {canManageOrgMembers} from "@/lib/permissions";

import type {ReactNode} from "react";

const OrgLayout = (): ReactNode => {
  const {slug} = useParams() as { slug: string };
  const {user} = useSession();
  const organization = api.useQuery("get", "/organizations/{login}", {
    params: {path: {login: slug}},
  });
  const membership = useQuery({
    queryKey: ["org-members-permission", slug, user?.id],
    queryFn: () => canManageOrgMembers(slug),
    enabled: Boolean(user),
  });
  if (organization.error) {
    return <Outlet />;
  }
  if (!organization.data) {
    return <QueryState error={organization.error} />;
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
