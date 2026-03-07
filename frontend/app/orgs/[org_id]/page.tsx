import { DefaultLayout } from '@/components/shared';
import { OrgPageActions, OrgTabs } from '@/components/orgs';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getContests, getOrganization, getProblems, listOrganizationMembers, listTeams } from '@/lib/actions';
import { Container, Group, Stack, Text, Title } from '@mantine/core';
import { notFound } from 'next/navigation';

type Props = { params: Promise<{ org_id: string }> };

export default async function OrgPage({ params }: Props) {
  const { org_id } = await params;

  const [orgError, orgData] = await getOrganization(org_id);
  if (orgError) {
    if (orgError.status === 404) notFound();
    return <DefaultLayout><Container size="lg" py="lg"><ErrorDisplay error={orgError} /></Container></DefaultLayout>;
  }

  const [
    [membersError, membersData],
    [teamsError, teamsData],
    [problemsError, problemsData],
    [contestsError, contestsData],
  ] = await Promise.all([
    listOrganizationMembers(org_id, 1, 20),
    listTeams(org_id, 1, 20),
    getProblems(1, 20, undefined, undefined, undefined, org_id),
    getContests(1, 20, undefined, org_id),
  ]);

  const org = orgData!.organization;
  const members = membersData?.members ?? [];
  const teams = teamsData?.teams ?? [];
  const problems = problemsData?.problems ?? [];
  const contests = contestsData?.contests ?? [];

  return (
    <DefaultLayout>
      <Container size="lg" py="lg">
        <Stack gap="md">
          <Group justify="space-between" align="flex-end">
            <div>
              <Title order={2}>{org.name}</Title>
              {org.description && <Text c="dimmed" size="sm">{org.description}</Text>}
            </div>
            <OrgPageActions orgId={org_id} />
          </Group>

          <OrgTabs
            members={members}
            teams={teams}
            problems={problems}
            contests={contests}
            orgId={org_id}
            membersError={membersError}
            teamsError={teamsError}
            problemsError={problemsError}
            contestsError={contestsError}
          />
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
