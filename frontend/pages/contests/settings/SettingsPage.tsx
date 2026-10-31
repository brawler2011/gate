import {Box, Container} from "@mantine/core";
import {IconPuzzle, IconSettings, IconUsers} from "@tabler/icons-react";
import {useLoaderData} from "react-router-dom";

import {MobileNav, SidebarNav} from "@/components/contests";
import {ParticipantsSection} from "@/components/contests/ParticipantsSection";
import {ProblemsSection} from "@/components/contests/ProblemsSection";
import {SettingsSection} from "@/components/contests/SettingsSection";
import {api, unwrap} from "@/lib/api";

import classes from "./styles.module.css";

import type {ContestProblemListItemModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};
  const {section = "settings"} = searchParams;

  const response = await unwrap(
    api.GET("/organizations/{org_login}/contests/{contest_login}", {
      params: {path: {org_login: slug, contest_login: contestLogin}},
    }),
  );
  return {slug, contestLogin, section, response};
}) satisfies LoaderFunction;

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

const ContestManagePage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, section, response} = data;
  const contest = response.contest;
  const problems: Array<ContestProblemListItemModel> = response.problems || [];
  const validSections = Object.values(SECTIONS);
  const activeSection = (validSections.includes(section as Section) ? section : SECTIONS.SETTINGS) as Section;
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <title>Настройки</title>
      <Box className={classes.manageLayout}>
        <SidebarNav activeSection={activeSection} sections={NAV_SECTIONS} />

        <Box className={classes.manageContent}>
          <MobileNav activeSection={activeSection} sections={NAV_SECTIONS} />

          <Box className={classes.contentPanel}>
            {activeSection === SECTIONS.SETTINGS && <SettingsSection contest={contest} />}
            {activeSection === SECTIONS.PROBLEMS && (
              <ProblemsSection orgLogin={slug} contestLogin={contestLogin} initialProblems={problems} />
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
