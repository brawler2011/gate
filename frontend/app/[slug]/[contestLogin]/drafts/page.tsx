import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import {DraftsClient} from "./DraftsClient";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Черновики решений",
  description: "Сохранение и просмотр рабочих версий решений задач контеста",
};

type PageProps = {
  params: Promise<{ slug: string; contestLogin: string }>;
  searchParams: Promise<{ problemId?: string; userId?: string }>;
};

const Page = async ({params, searchParams}: PageProps): Promise<ReactNode> => {
  const {slug, contestLogin} = await params;
  const queryParams = await searchParams;

  const [contestResponse, [, me]] = await Promise.all([
    unwrapAndCache(api.getContest)({orgLogin: slug, contestLogin}),
    api.getMe(),
  ]);

  if (!contestResponse?.contest) {
    return <ErrorDisplay error={{status: 404, message: "Контест не найден"}} />;
  }

  const user = me?.user ?? null;
  if (!user?.id) {
    redirect(`/${slug}/${contestLogin}`);
  }

  const contestRole = await getMyContestRole(slug, contestLogin);

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );

  const isManager = checker.canManageContest(contestResponse.contest);
  const hasStarted =
    !contestResponse.contest.start_time ||
    new Date(contestResponse.contest.start_time) <= new Date();

  if (!checker.canViewProblems(contestResponse.contest) || (!isManager && !hasStarted)) {
    redirect(`/${slug}/${contestLogin}`);
  }

  const isContestEnded = contestResponse.contest.end_time
    ? new Date(contestResponse.contest.end_time) <= new Date()
    : false;

  const [membersResponse, draftsResponse] = await Promise.all([
    isManager
      ? api.listContestMembers({orgLogin: slug, contestLogin, page: 1, pageSize: 100})
      : Promise.resolve([null, null] as const),
    api.listContestDrafts({
      orgLogin: slug,
      contestLogin,
      page: 1,
      pageSize: 50,
      userId: queryParams.userId || (isManager ? undefined : user.id),
      problemId: queryParams.problemId,
    }),
  ]);

  const members = membersResponse?.[1]?.members || [];
  const initialDrafts = draftsResponse?.[1]?.drafts || [];

  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}} pt="md">
      <DraftsClient
        contest={contestResponse.contest}
        problems={contestResponse.problems || []}
        user={user}
        isManager={isManager}
        isContestEnded={isContestEnded}
        initialDrafts={initialDrafts}
        members={members}
      />
    </Container>
  );
};

export default Page;
