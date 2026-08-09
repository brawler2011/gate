import {Container} from "@mantine/core";
import {notFound} from "next/navigation";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {buildOrgHeaderNav} from "@/lib/org-header-nav";
import {canManageOrgMembers} from "@/lib/org-permissions";

import type {Metadata} from "next";

type Props = {
  params: Promise<{ org_id: string }>;
};

export const metadata: Metadata = {
  title: "Участники",
};

const OrgMembersPage = async ({params}: Props) => {
  const {org_id} = await params;

  const canManage = await canManageOrgMembers(org_id);
  if (!canManage) {
    notFound();
  }

  const [orgError, orgData] = await api.getOrganization({id: org_id});
  if (orgError) {
    if (orgError.status === 404) {
      notFound();
    }
    return (
      <DefaultLayout headerOrganizationId={org_id}>
        <Container size="lg" py="lg">
          <ErrorDisplay error={orgError} />
        </Container>
      </DefaultLayout>
    );
  }

  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "members",
    showMembersTab: true,
  });
  const org = orgData!.organization;

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{id: org.id, name: org.name}}
    >
      <Container size="lg" py="lg">
        <OrgMembersManagement orgId={org_id} />
      </Container>
    </DefaultLayout>
  );
};

export default OrgMembersPage;
