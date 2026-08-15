import {Box, Container, Stack} from "@mantine/core";
import {notFound} from "next/navigation";

import {OrgDangerZone} from "@/components/orgs/OrgDangerZone";
import {OrgSettingsForm} from "@/components/orgs/OrgSettingsForm";
import {OrgSettingsMobileNav} from "@/components/orgs/OrgSettingsMobileNav";
import {ORG_SETTINGS_NAV_SECTIONS} from "@/components/orgs/OrgSettingsNavShared";
import {OrgSettingsSidebarNav} from "@/components/orgs/OrgSettingsSidebarNav";
import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {buildOrgHeaderNav} from "@/lib/org-header-nav";
import {canManageOrgMembers} from "@/lib/org-permissions";

import classes from "./styles.module.css";

const SECTIONS = {
  SETTINGS: "settings",
  DANGER: "danger",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ section?: string }>;
};

export const metadata: Metadata = {
  title: "Настройки",
};

const OrgSettingsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {org_id} = await params;
  const {section = "settings"} = await searchParams;

  const [error, data] = await api.getOrganization({id: org_id});
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    return (
      <DefaultLayout headerOrganizationId={org_id}>
        <Container size="sm" py="lg">
          <ErrorDisplay error={error} />
        </Container>
      </DefaultLayout>
    );
  }
  const org = data!.organization;

  const canManage = await canManageOrgMembers(org_id);
  if (!canManage) {
    notFound();
  }

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "settings",
    showMembersTab: canManage,
  });

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{id: org.id, name: org.name}}
    >
      <Container size="lg" py="lg">
        <Stack gap="md">
          <Box className={classes.manageLayout}>
            <OrgSettingsSidebarNav
              orgId={org_id}
              activeSection={activeSection}
              sections={ORG_SETTINGS_NAV_SECTIONS}
            />

            <Box className={classes.manageContent}>
              <OrgSettingsMobileNav
                orgId={org_id}
                activeSection={activeSection}
                sections={ORG_SETTINGS_NAV_SECTIONS}
              />

              <Box className={classes.contentPanel}>
                {activeSection === SECTIONS.SETTINGS && (
                  <OrgSettingsForm org={org} />
                )}
                {activeSection === SECTIONS.DANGER && (
                  <OrgDangerZone orgId={org_id} orgName={org.name} />
                )}
              </Box>
            </Box>
          </Box>
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default OrgSettingsPage;
