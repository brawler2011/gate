import {Box, Container, Stack} from "@mantine/core";
import {useLoaderData} from "react-router-dom";

import {OrgDangerZone} from "@/components/orgs/OrgDangerZone";
import {OrgSettingsForm} from "@/components/orgs/OrgSettingsForm";
import {OrgSettingsMobileNav} from "@/components/orgs/OrgSettingsMobileNav";
import {ORG_SETTINGS_NAV_SECTIONS} from "@/components/orgs/OrgSettingsNavShared";
import {OrgSettingsSidebarNav} from "@/components/orgs/OrgSettingsSidebarNav";
import {api, unwrap} from "@/lib/api";
import {canManageOrgMembers} from "@/lib/permissions";
import NotFound from "@/pages/NotFoundPage";

import classes from "./styles.module.css";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug} = routeParams as {slug: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const {section = "settings"} = searchParams;

  const data = await unwrap(
    api.GET("/organizations/{login}", {
      params: {path: {login: decoded}},
    }),
  );
  const org = data.organization;

  const canManage = await canManageOrgMembers(org.login);
  return {section, org, canManage};
}) satisfies LoaderFunction;

const SECTIONS = {
  SETTINGS: "settings",
  DANGER: "danger",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

const OrgSettingsPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {section, org, canManage} = data;
  if (!canManage) {
    return <NotFound />;
  }
  const validSections = Object.values(SECTIONS);
  const activeSection = (validSections.includes(section as Section) ? section : SECTIONS.SETTINGS) as Section;
  return (
    <Container size="lg" py="lg">
      <title>Настройки</title>
      <Stack gap="md">
        <Box className={classes.manageLayout}>
          <OrgSettingsSidebarNav
            orgLogin={org.login}
            activeSection={activeSection}
            sections={ORG_SETTINGS_NAV_SECTIONS}
          />

          <Box className={classes.manageContent}>
            <OrgSettingsMobileNav
              orgLogin={org.login}
              activeSection={activeSection}
              sections={ORG_SETTINGS_NAV_SECTIONS}
            />

            <Box className={classes.contentPanel}>
              {activeSection === SECTIONS.SETTINGS && <OrgSettingsForm org={org} />}
              {activeSection === SECTIONS.DANGER && <OrgDangerZone orgLogin={org.login} orgName={org.name} />}
            </Box>
          </Box>
        </Box>
      </Stack>
    </Container>
  );
};

export default OrgSettingsPage;
