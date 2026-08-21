import {Container} from "@mantine/core";
import {notFound, redirect} from "next/navigation";

import {OrgProblemsTab} from "@/components/orgs/OrgProblemsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ page?: string }>;
};

export const metadata: Metadata = {
  title: "Задачи",
};

const getOrganization = unwrapAndCache(api.getOrganization);

const OrgProblemsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
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

  const {page} = await searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}/problems`);
  }

  const orgData = await getOrganization({login: decoded});

  const [
    [problemsError, problemsData],
    [, me],
  ] = await Promise.all([
    api.listProblems({page: currentPage, pageSize: 20, organizationId: orgData.organization.id}),
    api.getMe(),
  ]);

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
