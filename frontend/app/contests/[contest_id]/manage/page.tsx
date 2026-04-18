import { MobileNav, SidebarNav } from "@/components/contests";
import { ParticipantsSection } from "@/components/contests/ParticipantsSection";
import { ProblemsSection } from "@/components/contests/ProblemsSection";
import { SettingsSection } from "@/components/contests/SettingsSection";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getContest } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import {
  CONTEST_CONTENT_MAX_WIDTH,
  CONTEST_INFO_PANEL_WIDTH,
} from "@/lib/constants";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";
import type { ContestProblemListItemModel } from "@contracts/core/v1";
import { Box, Container } from "@mantine/core";
import { IconPuzzle, IconSettings, IconUsers } from "@tabler/icons-react";
import layoutClasses from "../contestLayout.module.css";
import classes from "./styles.module.css";

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

export default async function ContestManagePage({
  params,
  searchParams,
}: Props) {
  const { contest_id: contestId } = await params;
  const { section = "settings" } = await searchParams;

  const [error, response] = await getContest(contestId);
  if (error) return <ErrorDisplay error={error} />;

  const contest = response!.contest;
  const problems: Array<ContestProblemListItemModel> = response!.problems || [];

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;

  const user = await getCurrentUser();
  const contestRole = user ? await getMyContestRole(contestId) : null;
  const contestHeaderNav = buildContestHeaderNav({
    contest,
    user,
    contestRole,
    activeTab: "manage",
  });

  return (
    <DefaultLayout headerSecondaryNavItems={contestHeaderNav}>
      <Box className={layoutClasses.contestContainer}>
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
        </Box>

        {/* Right Sidebar - Placeholder to maintain alignment with main contest page */}
        <Box style={{ width: CONTEST_INFO_PANEL_WIDTH }} visibleFrom="sm" />
      </Box>
    </DefaultLayout>
  );
}
