import {Box, Container, Stack} from "@mantine/core";

import {OrgDangerZone} from "@/components/orgs/OrgDangerZone";
import {OrgSettingsForm} from "@/components/orgs/OrgSettingsForm";
import {OrgSettingsMobileNav} from "@/components/orgs/OrgSettingsMobileNav";
import {ORG_SETTINGS_NAV_SECTIONS} from "@/components/orgs/OrgSettingsNavShared";
import {OrgSettingsSidebarNav} from "@/components/orgs/OrgSettingsSidebarNav";
import {api} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {canManageOrgMembers} from "@/lib/permissions";
import {notFound} from "@/lib/routeErrors";

import classes from "./styles.module.css";

import type {ReactNode} from "react";

const loadOrgSettingsPage = async ({params, searchParams}: Props) => {
  const {slug} = params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const {section = "settings"} = searchParams;

  const [error, data] = await api.getOrganization(decoded);
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    throw error;
  }
  const org = data!.organization;

  const canManage = await canManageOrgMembers(org.login);
  return {slug, decoded, section, error, data, org, canManage};
};

const SECTIONS = {
  SETTINGS: "settings",
  DANGER: "danger",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

type Props = {
  params: { slug: string };
  searchParams: { section?: string };
};

const OrgSettingsPage = ({params, searchParams}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/settings/page.tsx", params, searchParams], () =>
    loadOrgSettingsPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {section, org, canManage} = routeQuery.data;
  if (!canManage) {
    notFound();
  }
  const validSections = Object.values(SECTIONS);
  const activeSection = (validSections.includes(section as Section) ? section : SECTIONS.SETTINGS) as Section;
  return (
    <Container size="lg" py="lg">
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
