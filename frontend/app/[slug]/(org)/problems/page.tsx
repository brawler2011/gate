import {Container} from "@mantine/core";

import {OrgProblemsTab} from "@/components/orgs/OrgProblemsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {notFound, redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadOrgProblemsPage = async ({params, searchParams}: Props) => {
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

  const {page} = searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}/problems`);
  }

  const orgData = await getOrganization(decoded);

  const [[problemsError, problemsData], [, me]] = await Promise.all([
    api.listProblems({
      page: currentPage,
      pageSize: 20,
      organization_id: orgData.organization.id
    }),
    api.getMe(),
  ]);
  return {
    slug,
    decoded,
    page,
    currentPage,
    orgData,
    problemsError,
    problemsData,
    me,
  };
};

type Props = {
  params: { slug: string };
  searchParams: { page?: string };
};

const getOrganization = unwrap(api.getOrganization);

const OrgProblemsPage = ({params, searchParams}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/problems/page.tsx", params, searchParams], () =>
    loadOrgProblemsPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {orgData, problemsError, problemsData, me} = routeQuery.data;
  const currentUser = me?.user ?? null;
  const org = orgData.organization;
  const problems = problemsData?.problems ?? [];
  const problemsPagination = problemsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;
  return (
    <Container size="lg" py="lg">
      {problemsError ? (
        <ErrorDisplay error={problemsError} />
      ) : (
        <OrgProblemsTab
          problems={problems}
          pagination={problemsPagination}
          org={org}
          isAuthenticated={isAuthenticated}
        />
      )}
    </Container>
  );
};

export default OrgProblemsPage;
