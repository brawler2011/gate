import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Box,
  Container,
} from "@mantine/core";
import { redirect } from "next/navigation";

import { ContestInfoPanel } from "@/components/contests/ContestInfoPanel";
import { Layout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { Footer } from "@/components/shared/Footer";
import { HeaderWithSession } from "@/components/shared/HeaderWithSession";
import { api } from "@/lib/api";
import { unwrapAndCache } from "@/lib/api2";
import {
  CONTEST_CONTENT_MAX_WIDTH,
  CONTEST_INFO_PANEL_COMPACT_WIDTH,
} from "@/lib/constants";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";
import { PermissionChecker } from "@/lib/permissions";


import classes from "../contestLayout.module.css";
import { SubmitSubmissionClient } from "./SubmitSubmissionClient";

import type { Metadata } from "next";


const metadata: Metadata = {
  title: "Подать решение",
};

type PageProps = {
  params: Promise<{ contest_id: string }>;
};

const Page = async ({ params }: PageProps) => {
  const { contest_id } = await params;

  const response = await unwrapAndCache(api.getContest)({ contestId: contest_id });

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
          contest={{ id: response!.contest.id, title: response!.contest.title }}
        />
      </AppShellHeader>
      <AppShellMain>
        <Box maw="1920px" mx="auto" w="100%">
          <Box className={classes.contestContainerWithLeftInfo}>
            {/* Left Sidebar - Contest Info Panel - hidden on mobile */}
            <Box
              style={{ width: CONTEST_INFO_PANEL_COMPACT_WIDTH }}
              visibleFrom="sm"
            >
              <ContestInfoPanel
                contest={response.contest}
                user={user}
                width={CONTEST_INFO_PANEL_COMPACT_WIDTH}
              />
            </Box>

            {/* Main Content */}
            <Box style={{ width: CONTEST_CONTENT_MAX_WIDTH }}>
              <Container
                size="lg"
                pt={0}
                pb={{ base: "md", sm: "lg", md: "xl" }}
                px={0}
                mx={0}
                style={{ maxWidth: "100%" }}
              >
                <SubmitSubmissionClient
                  contest={response.contest}
                  problems={response.problems || []}
                  user={user}
                />
              </Container>
            </Box>
          </Box>
        </Box>
      </AppShellMain>
      <AppShellFooter withBorder={false}>
        <Footer />
      </AppShellFooter>
    </Layout>
  );
};

export default Page;
