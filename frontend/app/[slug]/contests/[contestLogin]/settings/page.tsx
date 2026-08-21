import {Box, Container} from "@mantine/core";
import {IconPuzzle, IconSettings, IconUsers} from "@tabler/icons-react";

import {MobileNav, SidebarNav} from "@/components/contests";
import {ParticipantsSection} from "@/components/contests/ParticipantsSection";
import {ProblemsSection} from "@/components/contests/ProblemsSection";
import {SettingsSection} from "@/components/contests/SettingsSection";
import {api, unwrapAndCache} from "@/lib/api";

import classes from "./styles.module.css";

import type {ContestProblemListItemModel} from "@/contracts/core/v1";
import type {Metadata} from "next";

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
  params: Promise<{ slug: string; contestLogin: string }>;
  searchParams: Promise<{ section?: string }>;
};

const ContestManagePage = async ({
  params,
  searchParams,
}: Props): Promise<JSX.Element> => {
  const {slug, contestLogin} = await params;
  const {section = "settings"} = await searchParams;

  const response = await unwrapAndCache(api.getContest)({orgLogin: slug, contestLogin});

  const contest = response.contest;
  const problems: Array<ContestProblemListItemModel> = response.problems || [];

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;

  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <Box className={classes.manageLayout}>
        <SidebarNav
          activeSection={activeSection}
          sections={NAV_SECTIONS}
        />

        <Box className={classes.manageContent}>
          <MobileNav
            activeSection={activeSection}
            sections={NAV_SECTIONS}
          />

          <Box className={classes.contentPanel}>
            {activeSection === SECTIONS.SETTINGS && (
              <SettingsSection contest={contest} />
            )}
            {activeSection === SECTIONS.PROBLEMS && (
              <ProblemsSection
                orgLogin={slug}
                contestLogin={contestLogin}
                initialProblems={problems}
              />
            )}
            {activeSection === SECTIONS.PARTICIPANTS && (
              <ParticipantsSection orgLogin={slug} contestLogin={contestLogin} />
            )}
          </Box>
        </Box>
      </Box>
    </Container>
  );
};

export default ContestManagePage;

