import { DefaultLayout } from '@/components/shared';
import { OrgSettingsForm } from '@/components/orgs/OrgSettingsForm';
import { OrgMembersManagement } from '@/components/orgs/OrgMembersManagement';
import { OrgDangerZone } from '@/components/orgs/OrgDangerZone';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getOrganization } from '@/lib/actions';
import { Container, Stack } from '@mantine/core';
import { notFound } from 'next/navigation';
import Link from 'next/link';
import { IconAlertTriangle, IconArrowLeft, IconSettings, IconUsers } from '@tabler/icons-react';
import classes from './styles.module.css';

const SECTIONS = {
  SETTINGS: 'settings',
  MEMBERS: 'members',
  DANGER: 'danger',
} as const;

type Section = typeof SECTIONS[keyof typeof SECTIONS];

const NAV_SECTIONS = [
  { key: SECTIONS.SETTINGS, label: 'Настройки', icon: IconSettings },
  { key: SECTIONS.MEMBERS, label: 'Участники', icon: IconUsers },
  { key: SECTIONS.DANGER, label: 'Опасная зона', icon: IconAlertTriangle },
] as const;

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ section?: string }>;
};

export default async function OrgSettingsPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { section = 'settings' } = await searchParams;

  const [error, data] = await getOrganization(org_id);
  if (error) {
    if (error.status === 404) notFound();
    return (
      <DefaultLayout>
        <Container size="sm" py="lg"><ErrorDisplay error={error} /></Container>
      </DefaultLayout>
    );
  }
  const org = data!.organization;

  const validSections = Object.values(SECTIONS);
  const activeSection = (
    validSections.includes(section as Section) ? section : SECTIONS.SETTINGS
  ) as Section;

  return (
    <DefaultLayout>
      <Container size="sm" py="lg">
        <Stack gap="md">
          <Link href={`/orgs/${org_id}`} className={classes.tab} style={{ alignSelf: 'flex-start' }}>
            <IconArrowLeft size={16} />
            Назад к организации
          </Link>

          <div>
            <div className={classes.tabRow}>
              {NAV_SECTIONS.map((s) => {
                const Icon = s.icon;
                const isActive = activeSection === s.key;
                return (
                  <Link
                    key={s.key}
                    href={`/orgs/${org_id}/settings?section=${s.key}`}
                    className={`${classes.tab} ${isActive ? classes.tabActive : ''}`}
                  >
                    <Icon size={16} />
                    {s.label}
                  </Link>
                );
              })}
            </div>

            <div className={classes.contentPanel}>
              {activeSection === SECTIONS.SETTINGS && (
                <OrgSettingsForm org={org} />
              )}
              {activeSection === SECTIONS.MEMBERS && (
                <OrgMembersManagement orgId={org_id} />
              )}
              {activeSection === SECTIONS.DANGER && (
                <OrgDangerZone orgId={org_id} orgName={org.name} />
              )}
            </div>
          </div>
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
