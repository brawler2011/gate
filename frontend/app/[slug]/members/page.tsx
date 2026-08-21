import {Container} from "@mantine/core";
import {notFound} from "next/navigation";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {canManageOrgMembers} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string }>;
};

export const metadata: Metadata = {
  title: "Участники",
};

const OrgMembersPage = async ({params}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const canManage = await canManageOrgMembers(decoded);
  if (!canManage) {
    notFound();
  }

  return (
    <Container size="lg" py="lg">
      <OrgMembersManagement orgLogin={decoded} />
    </Container>
  );
};

export default OrgMembersPage;
