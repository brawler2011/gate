import { DefaultLayout } from '@/components/shared';
import { TeamMembersManagement } from '@/components/orgs/TeamMembersManagement';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getTeam } from '@/lib/actions';
import { Container, Stack, Text, Title } from '@mantine/core';
import { notFound } from 'next/navigation';
import { IconArrowLeft } from '@tabler/icons-react';
import { LinkAnchor } from '@/components/shared';

type Props = { params: Promise<{ org_id: string; team_id: string }> };

export default async function TeamPage({ params }: Props) {
  const { org_id, team_id } = await params;
  const [error, data] = await getTeam(team_id);
  if (error) {
    if (error.status === 404) notFound();
    return (
      <DefaultLayout>
        <Container size="sm" py="lg"><ErrorDisplay error={error} /></Container>
      </DefaultLayout>
    );
  }
  const team = data!.team;

  return (
    <DefaultLayout>
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
}
