import {Alert, Container, Group, Paper, Stack} from "@mantine/core";
import {IconAlertCircle} from "@tabler/icons-react";
import {redirect} from "next/navigation";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {NextPagination} from "@/components/shared/Pagination";
import {SubmissionsListClient} from "@/components/submissions";
import {api, unwrapAndCache} from "@/lib/api";
import {env} from "@/lib/env";
import {parsePage} from "@/lib/lib";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Мои посылки",
  description: "",
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
  params: Promise<{ contest_id: string }>;
  searchParams: Promise<SearchParams>;
}

const PAGE_SIZE = 20;

const Page = async ({params, searchParams}: PageProps): Promise<ReactNode> => {
  const {contest_id} = await params;
  const queryParams = await searchParams;

  const page = parsePage(queryParams.page);
  if (!page) {
    redirect(`/contests/${contest_id}/mysubmissions`);
  }

  const parsedParams: {
    page: number;
    pageSize: number;
    contestId: string;
    userId?: string;
    problemId?: string;
    state?: number;
    sortOrder?: "asc" | "desc";
    language?: number;
  } = {
    page,
    pageSize: PAGE_SIZE,
    contestId: contest_id,
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

  const [error, submissionsData] = await api.listContestSubmissions(parsedParams);

  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (!submissionsData) {
    return (
      <Container size="lg" py="xl">
        <Alert
          icon={<IconAlertCircle size="1rem" />}
          title="Ошибка загрузки"
          color="red"
        >
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

  const contestData = await unwrapAndCache(api.getContest)({contestId: contest_id});

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  if (contestData?.contest) {
    const checker = new PermissionChecker(
      user,
      contestRole?.role ?? null,
      null,
      contestRole?.permissionsMask ?? null,
    );
    const isManager = checker.canManageContest(contestData.contest);
    const hasStarted =
      !contestData.contest.start_time ||
      new Date(contestData.contest.start_time) <= new Date();

    if (!checker.canViewMySubmissions(contestData.contest) || (!isManager && !hasStarted)) {
      redirect(`/contests/${contest_id}`);
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
            backgroundColor:
              "light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))",
            borderColor:
              "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
          }}
        >
          <Stack gap="md">
            <SubmissionsListClient
              initialSubmissions={submissionsData.submissions}
              wsUrl={wsBaseUrl + "/submissions"}
              since={submissionsData.since}
              snapshotScope="mine"
              filter={{
                contestId: contest_id,
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
                baseUrl={`/contests/${contest_id}/mysubmissions`}
                queryParams={nextQueryParams}
              />
            </Group>
          </Stack>
        </Paper>
      ) : (
        <ErrorDisplay
          error={{status: 404, message: "Contest not found"}}
        />
      )}
    </Container>
  );
};

export default Page;
