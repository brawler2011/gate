import {Container} from "@mantine/core";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {canManageOrgMembers} from "@/lib/permissions";
import {notFound} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadOrgMembersPage = async ({params}: Props) => {
  const {slug} = params;
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
  return {slug, decoded, canManage};
};

type Props = {
  params: { slug: string };
};

const OrgMembersPage = ({params}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/members/page.tsx", params], () =>
    loadOrgMembersPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {decoded, canManage} = routeQuery.data;
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
