import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import {SubmitSubmissionClient} from "./SubmitSubmissionClient";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Послать решение",
};

type PageProps = {
  params: Promise<{ contest_id: string }>;
};

const Page = async ({params}: PageProps): Promise<ReactNode> => {
  const {contest_id} = await params;

  const response = await unwrapAndCache(api.getContest)({contestId: contest_id});

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(response.contest);
  const hasStarted =
    !response.contest.start_time ||
    new Date(response.contest.start_time) <= new Date();

  if (!checker.canSubmitSolution(response.contest) || (!isManager && !hasStarted)) {
    redirect(`/contests/${contest_id}`);
  }

  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <SubmitSubmissionClient
        contest={response.contest}
        problems={response.problems || []}
        user={user}
      />
    </Container>
  );
};

export default Page;
