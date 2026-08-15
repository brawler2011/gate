import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Container,
} from "@mantine/core";
import {redirect} from "next/navigation";

import {Layout} from "@/components/shared";
import {Footer} from "@/components/shared/Footer";
import {HeaderWithSession} from "@/components/shared/HeaderWithSession";
import {api} from "@/lib/api";
import {unwrapAndCache} from "@/lib/api2";
import {buildContestHeaderNav} from "@/lib/contest-header-nav";
import {getMyContestRole} from "@/lib/contest-role";
import {PermissionChecker} from "@/lib/permissions";

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

  // Get user and contest role for permissions
  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  const checker = new PermissionChecker(user, contestRole?.role ?? null);
  const isManager = checker.canManageContest(response.contest);
  const hasStarted = !response.contest.start_time || new Date(response.contest.start_time) <= new Date();

  if (!isManager && !hasStarted) {
    redirect(`/contests/${contest_id}`);
  }

  const contestHeaderNav = buildContestHeaderNav({
    contest: response.contest,
    user,
    contestRole,
    activeTab: "submit",
  });

  return (
    <Layout>
      <AppShellHeader>
        <HeaderWithSession
          secondaryNavItems={contestHeaderNav}
          organizationId={response!.contest.organization_id}
          contest={{id: response!.contest.id, title: response!.contest.title}}
        />
      </AppShellHeader>
      <AppShellMain>
        <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
          <SubmitSubmissionClient
            contest={response.contest}
            problems={response.problems || []}
            user={user}
          />
        </Container>
      </AppShellMain>
      <AppShellFooter withBorder={false}>
        <Footer />
      </AppShellFooter>
    </Layout>
  );
};

export default Page;
