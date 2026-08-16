"use client";

import {Container, Paper, Tabs, Title} from "@mantine/core";
import {IconUser, IconUsersGroup} from "@tabler/icons-react";
import {useParams} from "next/navigation";
import {useState} from "react";

import {ProblemMembersManagement} from "@/components/problems/ProblemMembersManagement";
import {ProblemTeamsManagement} from "@/components/problems/ProblemTeamsManagement";

import type {ReactNode} from "react";

export default function ProblemAccessPage(): ReactNode {
  const params = useParams();
  const problemId = params?.problem_id as string;
  const [activeTab, setActiveTab] = useState<string | null>("users");

  return (
    <Container size="lg" py="xl">
      <Paper radius="md" p="xl" withBorder>
        <Title order={2} mb="lg">
          Права доступа к задаче
        </Title>

        <Tabs value={activeTab} onChange={setActiveTab}>
          <Tabs.List mb="md">
            <Tabs.Tab value="users" leftSection={<IconUser size={16} />}>
              Пользователи
            </Tabs.Tab>
            <Tabs.Tab value="teams" leftSection={<IconUsersGroup size={16} />}>
              Команды
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="users">
            <ProblemMembersManagement problemId={problemId} />
          </Tabs.Panel>

          <Tabs.Panel value="teams">
            <ProblemTeamsManagement problemId={problemId} />
          </Tabs.Panel>
        </Tabs>
      </Paper>
    </Container>
  );
}
