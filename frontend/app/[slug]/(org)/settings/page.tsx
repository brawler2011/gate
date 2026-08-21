import {Box, Container, Stack} from "@mantine/core";
import {notFound} from "next/navigation";

import {OrgDangerZone} from "@/components/orgs/OrgDangerZone";
import {OrgSettingsForm} from "@/components/orgs/OrgSettingsForm";
import {OrgSettingsMobileNav} from "@/components/orgs/OrgSettingsMobileNav";
import {ORG_SETTINGS_NAV_SECTIONS} from "@/components/orgs/OrgSettingsNavShared";
import {OrgSettingsSidebarNav} from "@/components/orgs/OrgSettingsSidebarNav";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {canManageOrgMembers} from "@/lib/permissions";

import classes from "./styles.module.css";

const SECTIONS = {
  SETTINGS: "settings",
  DANGER: "danger",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ section?: string }>;
};

export const metadata: Metadata = {
  title: "Настройки",
};

const OrgSettingsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const {section = "settings"} = await searchParams;

  const [error, data] = await api.getOrganization({login: decoded});
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    return (
      <Container size="sm" py="lg">
        <ErrorDisplay error={error} />
      </Container>
    );
  }
  const org = data!.organization;

  const canManage = await canManageOrgMembers(org.login);
  if (!canManage) {
    notFound();
  }

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;

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
              {activeSection === SECTIONS.SETTINGS && (
                <OrgSettingsForm org={org} />
              )}
              {activeSection === SECTIONS.DANGER && (
                <OrgDangerZone orgLogin={org.login} orgName={org.name} />
              )}
            </Box>
          </Box>
        </Box>
      </Stack>
    </Container>
  );
};

export default OrgSettingsPage;
