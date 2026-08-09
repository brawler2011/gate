import { Box, Container } from "@mantine/core";
import { IconPuzzle, IconSettings, IconUsers } from "@tabler/icons-react";

import { MobileNav, SidebarNav } from "@/components/contests";
import { ParticipantsSection } from "@/components/contests/ParticipantsSection";
import { ProblemsSection } from "@/components/contests/ProblemsSection";
import { SettingsSection } from "@/components/contests/SettingsSection";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { api } from "@/lib/api";
import { unwrapAndCache } from "@/lib/api2";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";


import classes from "./styles.module.css";

import type { ContestProblemListItemModel } from "@/contracts/core/v1";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Настройки",
};


// Constants for sections
const SECTIONS = {
  SETTINGS: "settings",
  PROBLEMS: "problems",
  PARTICIPANTS: "participants",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

// Navigation configuration
const NAV_SECTIONS = [
  {
    key: SECTIONS.SETTINGS,
    label: "Настройки",
    icon: IconSettings,
  },
  {
    key: SECTIONS.PROBLEMS,
    label: "Задачи",
    icon: IconPuzzle,
  },
  {
    key: SECTIONS.PARTICIPANTS,
    label: "Участники",
    icon: IconUsers,
  },
] as const;

type Props = {
  params: Promise<{ contest_id: string }>;
  searchParams: Promise<{ section?: string }>;
};

const ContestManagePage = async ({
  params,
  searchParams,
}: Props) => {
  const { contest_id: contestId } = await params;
  const { section = "settings" } = await searchParams;

  const response = await unwrapAndCache(api.getContest)({ contestId });

  const contest = response.contest;
  const problems: Array<ContestProblemListItemModel> = response.problems || [];

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contestId) : null;
  const contestHeaderNav = buildContestHeaderNav({
    contest,
    user,
    contestRole,
    activeTab: "settings",
  });

  return (
    <DefaultLayout
      headerSecondaryNavItems={contestHeaderNav}
      headerOrganizationId={contest.organization_id}
      headerContest={{ id: contest.id, title: contest.title }}
    >
      <Container size="lg" pb={{ base: "md", sm: "lg", md: "xl" }}>
        <Box className={classes.manageLayout}>
          <SidebarNav
            contestId={contestId}
            activeSection={activeSection}
            sections={NAV_SECTIONS}
          />

          <Box className={classes.manageContent}>
            <MobileNav
              contestId={contestId}
              activeSection={activeSection}
              sections={NAV_SECTIONS}
            />

            <Box className={classes.contentPanel}>
              {activeSection === SECTIONS.SETTINGS && (
                <SettingsSection contest={contest} />
              )}
              {activeSection === SECTIONS.PROBLEMS && (
                <ProblemsSection
                  contestId={contestId}
                  initialProblems={problems}
                />
              )}
              {activeSection === SECTIONS.PARTICIPANTS && (
                <ParticipantsSection contestId={contestId} />
              )}
            </Box>
          </Box>
        </Box>
      </Container>
    </DefaultLayout>
  );
};

export default ContestManagePage;
