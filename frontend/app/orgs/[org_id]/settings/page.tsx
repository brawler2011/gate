import { OrgDangerZone } from "@/components/orgs/OrgDangerZone";
import { OrgMembersManagement } from "@/components/orgs/OrgMembersManagement";
import { OrgSettingsForm } from "@/components/orgs/OrgSettingsForm";
import { OrgSettingsMobileNav } from "@/components/orgs/OrgSettingsMobileNav";
import { ORG_SETTINGS_NAV_SECTIONS } from "@/components/orgs/OrgSettingsNavShared";
import { OrgSettingsSidebarNav } from "@/components/orgs/OrgSettingsSidebarNav";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getOrganization } from "@/lib/actions";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { Box, Container, Stack } from "@mantine/core";
import { notFound } from "next/navigation";
import classes from "./styles.module.css";

const SECTIONS = {
  SETTINGS: "settings",
  MEMBERS: "members",
  DANGER: "danger",
} as const;

type Section = (typeof SECTIONS)[keyof typeof SECTIONS];

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ section?: string }>;
};

export default async function OrgSettingsPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { section = "settings" } = await searchParams;

  const [error, data] = await getOrganization(org_id);
  if (error) {
    if (error.status === 404) notFound();
    return (
      <DefaultLayout>
        <Container size="sm" py="lg">
          <ErrorDisplay error={error} />
        </Container>
      </DefaultLayout>
    );
  }
  const org = data!.organization;

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "settings",
  });

  return (
    <DefaultLayout headerSecondaryNavItems={orgHeaderNav}>
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
                {activeSection === SECTIONS.MEMBERS && (
                  <OrgMembersManagement orgId={org_id} />
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
}
