import {Container, Paper, Stack, Tabs, Text, Title} from "@mantine/core";
import {IconArrowLeft, IconFileCode, IconTrophy, IconUsers} from "@tabler/icons-react";
import {notFound} from "next/navigation";

import {TeamContestsTab} from "@/components/orgs/TeamContestsTab";
import {TeamMembersManagement} from "@/components/orgs/TeamMembersManagement";
import {TeamProblemsTab} from "@/components/orgs/TeamProblemsTab";
import {DefaultLayout, LinkAnchor} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = { params: Promise<{ org_id: string; team_id: string }> };

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {team_id} = await params;
  const [error, data] = await api.getTeam({id: team_id});
  if (error || !data) {
    return {title: "Команда"};
  }
  return {title: data.team.name};
};

const TeamPage = async ({params}: Props): Promise<ReactNode> => {
  const {org_id, team_id} = await params;
  const [error, data] = await api.getTeam({id: team_id});
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
  const team = data!.team;

  return (
    <DefaultLayout headerOrganizationId={org_id}>
      <Container size="md" py="lg">
        <Stack gap="xl">
          <LinkAnchor href={`/orgs/${org_id}`} size="sm">
            <IconArrowLeft size={14} style={{marginRight: 4}} />
            Назад к организации
          </LinkAnchor>

          <div>
            <Title order={2}>{team.name}</Title>
            {team.description && (
              <Text c="dimmed" size="sm">
                {team.description}
              </Text>
            )}
          </div>

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
    </DefaultLayout>
  );
};

export default TeamPage;
