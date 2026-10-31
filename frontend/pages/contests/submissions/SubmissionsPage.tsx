import {Alert, Container, Group, Paper, Stack} from "@mantine/core";
import {IconAlertCircle} from "@tabler/icons-react";
import {useLoaderData, redirect} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {RoutePagination} from "@/components/shared/Pagination";
import {SubmissionsListClient} from "@/components/submissions";
import {api, unwrap} from "@/lib/api";
import {env} from "@/lib/env";
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
    sortOrder?: "asc" | "desc";
    language?: number;
  } = {
    page,
    pageSize: PAGE_SIZE,
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

  const {data: submissionsData, error: submissionsErr, response: submissionsResponse} = await api.GET(
    "/organizations/{org_login}/contests/{contest_login}/submissions",
    {
      params: {
        path: {org_login: slug, contest_login: contestLogin},
        query: parsedParams,
      },
    },
  );
  const error = submissionsErr
    ? {
      status: submissionsResponse.status,
      message: submissionsErr.message || submissionsResponse.statusText,
    }
    : null;
  return {slug, contestLogin, parsedParams, contestData, canRejudge, error, submissionsData};
}) satisfies LoaderFunction;

const PAGE_SIZE = 15;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, parsedParams, contestData, canRejudge, error, submissionsData} =
    data;
  if (error) {
    return <ErrorDisplay error={error} />;
  }
  if (!submissionsData) {
    return (
      <Container size="lg" py="xl">
        <Alert icon={<IconAlertCircle size="1rem" />} title="Ошибка загрузки" color="red">
          Не удалось загрузить список решений. Попробуйте обновить страницу.
        </Alert>
      </Container>
    );
  }
  const nextQueryParams: Record<string, string | number | undefined> = {
    page: parsedParams.page,
    pageSize: parsedParams.pageSize,
    userId: parsedParams.userId,
    problemId: parsedParams.problemId,
    state: parsedParams.state,
    order: parsedParams.sortOrder,
    language: parsedParams.language,
  };
  const wsBaseUrl = env.getWebSocketUrl();
  return (
    <Container size="lg" pb="xl">
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
              initialSubmissions={submissionsData.submissions}
              wsUrl={wsBaseUrl + "/submissions"}
              since={submissionsData.since}
              snapshotScope="all"
              filter={{
                orgLogin: slug,
                contestLogin,
                userId: parsedParams.userId,
                problemId: parsedParams.problemId,
              }}
              pageSize={PAGE_SIZE}
              page={parsedParams.page}
              sortOrder={parsedParams.sortOrder}
              canRejudge={canRejudge}
            />
            <Group justify="center">
              <RoutePagination
                pagination={submissionsData.pagination}
                baseUrl={`/${slug}/${contestLogin}/submissions`}
                queryParams={nextQueryParams}
              />
            </Group>
          </Stack>
        </Paper>
      ) : (
        <ErrorDisplay error={{status: 404, message: "Contest not found"}} />
      )}
    </Container>
  );
};

export default Page;
