import {OrgHeaderNav} from "@/components/orgs";
import {DefaultLayout} from "@/components/shared";
import {api} from "@/lib/api";
import {canManageOrgMembers} from "@/lib/permissions";

import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ slug: string }>;
};

const OrgLayout = async ({
  children,
  params,
}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    return children;
  }

  const [orgError, orgData] = await api.getOrganization({login: decoded});
  if (orgError || !orgData) {
    // If not found or error, let child page handle notFound/error
    return children;
  }

  const [, me] = await api.getMe();
  const org = orgData.organization;
  const showMembersTab = await canManageOrgMembers(org.login);

  return (
    <DefaultLayout
      headerUser={me?.user ?? null}
      headerOrganization={{id: org.id, login: org.login, name: org.name}}
      headerSecondaryNav={
        <OrgHeaderNav
          orgLogin={org.login}
          showMembersTab={showMembersTab}
        />
      }
    >
      {children}
    </DefaultLayout>
  );
};

export default OrgLayout;
