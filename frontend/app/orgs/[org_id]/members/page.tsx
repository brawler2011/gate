import {Container} from "@mantine/core";
import {notFound} from "next/navigation";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {canManageOrgMembers} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ org_id: string }>;
};

export const metadata: Metadata = {
  title: "Участники",
};

const OrgMembersPage = async ({params}: Props): Promise<ReactNode> => {
  const {org_id} = await params;

  const canManage = await canManageOrgMembers(org_id);
  if (!canManage) {
    notFound();
  }

  return (
    <Container size="lg" py="lg">
      <OrgMembersManagement orgId={org_id} />
    </Container>
  );
};

export default OrgMembersPage;
