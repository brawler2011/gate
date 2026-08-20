import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {ContestMonitorTable} from "@/components/contests";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole} from "@/lib/contest-role";
import {PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Положение",
};

type PageProps = {
  params: Promise<{ contest_id: string }>;
};

const Page = async ({params}: PageProps): Promise<ReactNode> => {
  const {contest_id} = await params;

  // Fetch contest data and scoreboard
  const contestResponse = await unwrapAndCache(api.getContest)({contestId: contest_id});
  const scoreboard = await unwrapAndCache(api.getContestScoreboard)({contestId: contest_id});
  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  let isManager = false;
  if (contestResponse?.contest) {
    const checker = new PermissionChecker(
      user,
      contestRole?.role ?? null,
      null,
      contestRole?.permissionsMask ?? null,
    );
    isManager = checker.canManageContest(contestResponse.contest);
    const hasStarted =
      !contestResponse.contest.start_time ||
      new Date(contestResponse.contest.start_time) <= new Date();

    if (!checker.canViewMonitor(contestResponse.contest) || (!isManager && !hasStarted)) {
      redirect(`/contests/${contest_id}`);
    }
  }

  return (
    <Container size="lg" py="md">
      {scoreboard && (
        <ContestMonitorTable
          contestId={contest_id}
          initialScoreboard={scoreboard}
          startTime={contestResponse?.contest.start_time}
          endTime={contestResponse?.contest.end_time}
          isManager={isManager}
        />
      )}
    </Container>
  );
};

export default Page;
