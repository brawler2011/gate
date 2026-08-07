import { Container, Stack, Text, Title } from '@mantine/core';
import { IconArrowLeft } from '@tabler/icons-react';
import { notFound } from 'next/navigation';

import { TeamMembersManagement } from '@/components/orgs/TeamMembersManagement';
import { DefaultLayout } from '@/components/shared';
import { LinkAnchor } from '@/components/shared';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { api } from '@/lib/api';



type Props = { params: Promise<{ org_id: string; team_id: string }> };

const TeamPage = async ({ params }: Props) => {
  const { org_id, team_id } = await params;
  const [error, data] = await api.getTeam({ id: team_id });
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    return (
      <DefaultLayout headerOrganizationId={org_id}>
        <Container size="sm" py="lg"><ErrorDisplay error={error} /></Container>
      </DefaultLayout>
    );
  }
  const team = data!.team;

  return (
    <DefaultLayout headerOrganizationId={org_id}>
      <Container size="sm" py="lg">
        <Stack gap="xl">
          <LinkAnchor href={`/orgs/${org_id}`} size="sm">
            <IconArrowLeft size={14} style={{ marginRight: 4 }} />
            Назад к организации
          </LinkAnchor>

          <div>
            <Title order={2}>{team.name}</Title>
            {team.description && <Text c="dimmed" size="sm">{team.description}</Text>}
          </div>

          <TeamMembersManagement teamId={team_id} />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default TeamPage;
