"use client";

import {Container, Tabs} from "@mantine/core";
import {IconUser, IconUsersGroup} from "@tabler/icons-react";
import {useState} from "react";

import {ProblemMembersManagement} from "@/components/problems/ProblemMembersManagement";
import {ProblemTeamsManagement} from "@/components/problems/ProblemTeamsManagement";

import type {ReactNode} from "react";

type Props = {
  problemId: string;
};

export const WorkshopAccessTab = ({problemId}: Props): ReactNode => {
  const [activeTab, setActiveTab] = useState<string | null>("users");

  return (
    <Container size="lg" py="lg">
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
    </Container>
  );
};
