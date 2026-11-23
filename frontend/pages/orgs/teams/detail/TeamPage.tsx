import {Avatar, Badge, Container, Group, Paper, Stack, Tabs, Text, Title} from "@mantine/core";
import {IconArrowLeft, IconTrophy, IconUsers} from "@tabler/icons-react";
import {useLoaderData} from "react-router-dom";

import {TeamContestsTab} from "@/components/orgs/TeamContestsTab";
import {TeamMembersManagement} from "@/components/orgs/TeamMembersManagement";
import {LinkAnchor} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {JoinLinkCard} from "@/components/shared/JoinLinkCard";
import {api, unwrap} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug, team_id} = routeParams as {slug: string; team_id: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const {data, error, response} = await api.GET("/teams/{id}", {
    params: {path: {id: team_id}},
  });
  const err = error
    ? {
      status: response.status,
      message: (error as {message?: string})?.message || response.statusText,
    }
    : null;
  return {team_id, decoded, error: err, data};
}) satisfies LoaderFunction;

const TeamPage = (): ReactNode => {
  const loaded = useLoaderData<typeof loader>();
  const {team_id, decoded, error, data} = loaded;
  if (error) {
    if (error.status === 404) {
      return <NotFound />;
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
      <title>{team.name}</title>
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

        {team.permissions.manage && (
          <JoinLinkCard
            queryKey={["team-join-link", team_id]}
            load={() => unwrap(api.GET("/teams/{id}/join-link", {params: {path: {id: team_id}}}))}
            reset={() => unwrap(api.PUT("/teams/{id}/join-link", {params: {path: {id: team_id}}}))}
            disable={async () => {
              const {error} = await api.DELETE("/teams/{id}/join-link", {params: {path: {id: team_id}}});
              if (error) {
                throw new Error(problemMessage(error, "Не удалось выключить ссылку"));
              }
            }}
            description="Кто откроет ссылку, вступит в эту команду и в организацию, даже если в организацию принимают только по приглашению."
          />
        )}

        <Paper radius="md" p="md" withBorder>
          <Tabs defaultValue="members">
            <Tabs.List mb="md">
              <Tabs.Tab value="members" leftSection={<IconUsers size={16} />}>
                Участники
              </Tabs.Tab>
              <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
                Контесты
              </Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="members">
              <TeamMembersManagement teamId={team_id} />
            </Tabs.Panel>

            <Tabs.Panel value="contests">
              <TeamContestsTab teamId={team_id} />
            </Tabs.Panel>
          </Tabs>
        </Paper>
      </Stack>
    </Container>
  );
};

export default TeamPage;
