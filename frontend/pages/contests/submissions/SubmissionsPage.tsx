import {Container, Paper, Stack} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

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
    throw redirect(`/${slug}/${contestLogin}/submissions`);
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

  if (queryParams.userId) {
    parsedParams.userId = queryParams.userId;
  }
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
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;

  const checker = contestData?.contest
    ? new PermissionChecker(user, contestRole?.role ?? null, null, contestRole?.permissionsMask ?? null)
    : null;

  if (contestData?.contest && checker) {
    const isManager = checker.canManageContest(contestData.contest);
    const hasStarted =
      !contestData.contest.start_time || new Date(contestData.contest.start_time) <= new Date();

    if (!checker.canViewAllSubmissions(contestData.contest) || (!isManager && !hasStarted)) {
      throw redirect(`/${slug}/${contestLogin}`);
    }
  }

  const canRejudge =
    contestData?.contest && checker ? checker.canRejudgeSubmissions(contestData.contest) : false;

  const selection = {orgLogin: slug, contestLogin, contestId: contestData.contest.id, ...parsedParams};
  return {slug, contestLogin, contestData, canRejudge, selection};
}) satisfies LoaderFunction;

const PAGE_SIZE = 15;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {contestData, canRejudge, selection} =
    data;
  return (
    <Container size="lg" pb="xl">
      <title>Посылки</title>
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
              canRejudge={canRejudge}
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
