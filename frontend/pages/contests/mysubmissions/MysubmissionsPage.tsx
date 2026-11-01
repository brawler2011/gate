import {Container, Paper, Stack} from "@mantine/core";
import {useLoaderData, redirect, Navigate} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {SubmissionsListClient} from "@/components/submissions";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};
  const queryParams = searchParams;

  const page = parsePage(queryParams.page);
  if (!page) {
    throw redirect(`/${slug}/${contestLogin}/mysubmissions`);
  }

  const parsedParams: {
    page: number;
    pageSize: number;
    userId?: string;
    problemId?: string;
    state?: number;
    sortOrder: "asc" | "desc";
    language?: number;
  } = {
    page,
    pageSize: PAGE_SIZE,
    sortOrder: "desc",
  };

  if (queryParams.problemId) {
    parsedParams.problemId = queryParams.problemId;
  }
  if (queryParams.state) {
    parsedParams.state = Number(queryParams.state);
  }
  if (queryParams.order === "asc" || queryParams.order === "desc") {
    parsedParams.sortOrder = queryParams.order;
  }
  if (queryParams.language) {
    parsedParams.language = Number(queryParams.language);
  }

  const [contestData, {data: me}] = await Promise.all([
    unwrap(
      api.GET("/organizations/{org_login}/contests/{contest_login}", {
        params: {path: {org_login: slug, contest_login: contestLogin}},
      }),
    ),
    api.GET("/users/me"),
  ]);
  const user = me?.user ?? null;
  if (!user) {
    throw redirect('/auth/login');
  }
  parsedParams.userId = user.id;
  const selection = {orgLogin: slug, contestLogin, contestId: contestData.contest.id, ...parsedParams};
  const contestRole = await getMyContestRole(slug, contestLogin);
  return {
    slug, contestLogin, contestData, user, contestRole, selection,
  };
}) satisfies LoaderFunction;

const PAGE_SIZE = 20;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {
    slug,
    contestLogin,
    selection,
    contestData,
    user,
    contestRole,
  } = data;
  if (contestData?.contest) {
    const checker = new PermissionChecker(
      user,
      contestRole?.role ?? null,
      null,
      contestRole?.permissionsMask ?? null,
    );
    const isManager = checker.canManageContest(contestData.contest);
    const hasStarted =
      !contestData.contest.start_time || new Date(contestData.contest.start_time) <= new Date();

    if (!checker.canViewMySubmissions(contestData.contest) || (!isManager && !hasStarted)) {
      return <Navigate to={`/${slug}/${contestLogin}`} replace />;
    }
  }
  return (
    <Container size="lg" pb="xl">
      <title>Мои посылки</title>
      {contestData?.contest ? (
        <Paper
          withBorder
          p="md"
          w="100%"
          shadow="sm"
          radius="md"
          style={{
            backgroundColor: "light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))",
            borderColor: "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
          }}
        >
          <Stack gap="md">
            <SubmissionsListClient
              selection={selection}
            />

          </Stack>
        </Paper>
      ) : (
        <ErrorDisplay error={{status: 404, message: "Contest not found"}} />
      )}
    </Container>
  );
};

export default Page;
