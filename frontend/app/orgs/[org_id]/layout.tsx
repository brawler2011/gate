import {OrgHeaderNav} from "@/components/orgs";
import {DefaultLayout} from "@/components/shared";
import {api, unwrapAndCache} from "@/lib/api";
import {canManageOrgMembers} from "@/lib/permissions";

import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ org_id: string }>;
};

const OrgLayout = async ({
  children,
  params,
}: Props): Promise<ReactNode> => {
  const {org_id} = await params;
  const orgData = await unwrapAndCache(api.getOrganization)({id: org_id});
  const showMembersTab = await canManageOrgMembers(org_id);
  const org = orgData.organization;

  return (
    <DefaultLayout
      headerOrganization={{id: org.id, name: org.name}}
      headerSecondaryNav={
        <OrgHeaderNav
          orgId={org.id}
          showMembersTab={showMembersTab}
        />
      }
    >
      {children}
    </DefaultLayout>
  );
};

export default OrgLayout;
