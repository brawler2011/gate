import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {OrgProblemsTab} from "@/components/orgs/OrgProblemsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug} = routeParams as {slug: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const {page} = searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    throw redirect(`/${decoded}/problems`);
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
  return {orgData, problemsError, problemsData, me};
}) satisfies LoaderFunction;

const getOrganization = unwrap(api.getOrganization);

const OrgProblemsPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {orgData, problemsError, problemsData, me} = data;
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
