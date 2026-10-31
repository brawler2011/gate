import {Container, Group, Paper, Stack} from "@mantine/core";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {NextPagination} from "@/components/shared/Pagination";
import {SubmissionsListClient} from "@/components/submissions";
import {api, unwrap} from "@/lib/api";
import {env} from "@/lib/env";
import {parsePage} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";
import {redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadPage = async ({params, searchParams}: PageProps) => {
  const {slug, contestLogin} = params;
  const queryParams = searchParams;

  const page = parsePage(queryParams.page);
  if (!page) {
    redirect(`/${slug}/${contestLogin}/mysubmissions`);
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

  const [error, submissionsData] = await api.listContestSubmissions(slug, contestLogin, parsedParams);

  if (error) {
    throw error;
  }

  if (!submissionsData) {
    throw {status: 404, message: "Страница не найдена"};
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

  const contestData = await unwrap(api.getContest)(slug, contestLogin);

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;
  return {
    slug,
    contestLogin,
    queryParams,
    page,
    parsedParams,
    error,
    submissionsData,
    nextQueryParams,
    wsBaseUrl,
    contestData,
    me,
    user,
    contestRole,
  };
};

interface SearchParams {
  page?: string;
  userId?: string;
  problemId?: string;
  state?: string;
  order?: string;
  language?: string;
}

interface PageProps {
  params: { slug: string; contestLogin: string };
  searchParams: SearchParams;
}

const PAGE_SIZE = 20;

const Page = ({params, searchParams}: PageProps): ReactNode => {
  const routeQuery = usePageQuery(
    ["frontend/app/[slug]/[contestLogin]/mysubmissions/page.tsx", params, searchParams],
    () => loadPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {
    slug,
    contestLogin,
    parsedParams,
    submissionsData,
    nextQueryParams,
    wsBaseUrl,
    contestData,
    user,
    contestRole,
  } = routeQuery.data;
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
      redirect(`/${slug}/${contestLogin}`);
    }
  }
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
              snapshotScope="mine"
              filter={{
                orgLogin: slug,
                contestLogin,
                userId: parsedParams.userId,
                problemId: parsedParams.problemId,
              }}
              pageSize={PAGE_SIZE}
              page={parsedParams.page}
              sortOrder={parsedParams.sortOrder}
            />
            <Group justify="center">
              <NextPagination
                pagination={submissionsData.pagination}
                baseUrl={`/${slug}/${contestLogin}/mysubmissions`}
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
