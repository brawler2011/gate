import {Avatar, Badge, Container, Group, Paper, Stack, Tabs, Text, Title} from "@mantine/core";
import {IconArrowLeft, IconFileCode, IconTrophy, IconUsers} from "@tabler/icons-react";

import {TeamContestsTab} from "@/components/orgs/TeamContestsTab";
import {TeamMembersManagement} from "@/components/orgs/TeamMembersManagement";
import {TeamProblemsTab} from "@/components/orgs/TeamProblemsTab";
import {LinkAnchor} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {notFound} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadTeamPage = async ({params}: Props) => {
  const {slug, team_id} = params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const [error, data] = await api.getTeam(team_id);
  return {slug, team_id, decoded, error, data};
};

type Props = { params: { slug: string; team_id: string } };

const TeamPage = ({params}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/teams/[team_id]/page.tsx", params], () =>
    loadTeamPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {team_id, decoded, error, data} = routeQuery.data;
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
  const team = data!.team;
  return (
    <Container size="md" py="lg">
      <Stack gap="xl">
        <LinkAnchor href={`/${decoded}`} size="sm">
          <IconArrowLeft size={14} style={{marginRight: 4}} />
          Назад к организации
        </LinkAnchor>

        <Group wrap="nowrap">
          <Avatar size="lg" radius="xl" color="blue" aria-label={`Значок команды ${team.name}`}>
            {team.name.trim().slice(0, 2).toUpperCase()}
          </Avatar>
          <Stack gap="xs">
            <Group gap="sm">
              <Title order={2}>{team.name}</Title>
              <Badge variant="light">Команда</Badge>
            </Group>
            {team.description && (
              <Text c="dimmed" size="sm">
                {team.description}
              </Text>
            )}
          </Stack>
        </Group>

        <Paper radius="md" p="md" withBorder>
          <Tabs defaultValue="members">
            <Tabs.List mb="md">
              <Tabs.Tab value="members" leftSection={<IconUsers size={16} />}>
                Участники
              </Tabs.Tab>
              <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
                Контесты
              </Tabs.Tab>
              <Tabs.Tab value="problems" leftSection={<IconFileCode size={16} />}>
                Задачи
              </Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="members">
              <TeamMembersManagement teamId={team_id} />
            </Tabs.Panel>

            <Tabs.Panel value="contests">
              <TeamContestsTab teamId={team_id} />
            </Tabs.Panel>

            <Tabs.Panel value="problems">
              <TeamProblemsTab teamId={team_id} />
            </Tabs.Panel>
          </Tabs>
        </Paper>
      </Stack>
    </Container>
  );
};

export default TeamPage;
