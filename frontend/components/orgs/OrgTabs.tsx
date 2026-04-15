"use client";

import { OrgContestsTab } from "@/components/orgs/OrgContestsTab";
import { OrgMembersTab } from "@/components/orgs/OrgMembersTab";
import { OrgProblemsTab } from "@/components/orgs/OrgProblemsTab";
import { OrgTeamsTab } from "@/components/orgs/OrgTeamsTab";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import type { ApiError } from "@/lib/api";
import type {
  ContestModel,
  OrganizationMemberModel,
  ProblemsListItemModel,
  TeamModel,
} from "@contracts/gateway/v1";
import { Tabs } from "@mantine/core";
import {
  IconPuzzle,
  IconTrophy,
  IconUsers,
  IconUsersGroup,
} from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

const VALID_TABS = ["contests", "problems", "teams", "members"] as const;
type TabValue = (typeof VALID_TABS)[number];

function isValidTab(value: string | null): value is TabValue {
  return VALID_TABS.includes(value as TabValue);
}

type Props = {
  members: OrganizationMemberModel[];
  teams: TeamModel[];
  problems: ProblemsListItemModel[];
  contests: ContestModel[];
  orgId: string;
  membersError: ApiError | null;
  teamsError: ApiError | null;
  problemsError: ApiError | null;
  contestsError: ApiError | null;
};

export function OrgTabs({
  members,
  teams,
  problems,
  contests,
  orgId,
  membersError,
  teamsError,
  problemsError,
  contestsError,
}: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();

  const tabParam = searchParams.get("tab");
  const activeTab: TabValue = isValidTab(tabParam) ? tabParam : "contests";

  const handleTabChange = useCallback(
    (value: string | null) => {
      if (!value) return;
      const params = new URLSearchParams(searchParams.toString());
      params.set("tab", value);
      router.replace(`?${params.toString()}`, { scroll: false });
    },
    [router, searchParams],
  );

  return (
    <Tabs value={activeTab} onChange={handleTabChange} keepMounted={false}>
      <Tabs.List>
        <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
          Контесты ({contests.length})
        </Tabs.Tab>
        <Tabs.Tab value="problems" leftSection={<IconPuzzle size={16} />}>
          Задачи ({problems.length})
        </Tabs.Tab>
        <Tabs.Tab value="teams" leftSection={<IconUsersGroup size={16} />}>
          Команды ({teams.length})
        </Tabs.Tab>
        <Tabs.Tab value="members" leftSection={<IconUsers size={16} />}>
          Участники ({members.length})
        </Tabs.Tab>
      </Tabs.List>

      <Tabs.Panel value="contests" pt="md">
        {contestsError ? (
          <ErrorDisplay error={contestsError} />
        ) : (
          <OrgContestsTab contests={contests} />
        )}
      </Tabs.Panel>
      <Tabs.Panel value="problems" pt="md">
        {problemsError ? (
          <ErrorDisplay error={problemsError} />
        ) : (
          <OrgProblemsTab problems={problems} />
        )}
      </Tabs.Panel>
      <Tabs.Panel value="teams" pt="md">
        {teamsError ? (
          <ErrorDisplay error={teamsError} />
        ) : (
          <OrgTeamsTab teams={teams} orgId={orgId} />
        )}
      </Tabs.Panel>
      <Tabs.Panel value="members" pt="md">
        {membersError ? (
          <ErrorDisplay error={membersError} />
        ) : (
          <OrgMembersTab members={members} />
        )}
      </Tabs.Panel>
    </Tabs>
  );
}
