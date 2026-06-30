"use client";

import { ProblemCard } from "@/components/problems/ProblemCard";
import { NextPagination } from "@/components/shared/Pagination";
import { CreateProblemModal } from "@/components/workshop/CreateProblemModal";
import type {
  OrganizationModel,
  PaginationModel,
  ProblemsListItemModel,
} from "@contracts/core/v1";
import {
  Button,
  Center,
  Group,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { IconPlus, IconSearch } from "@tabler/icons-react";
import { useMemo, useState } from "react";

type Props = {
  problems: ProblemsListItemModel[];
  pagination: PaginationModel;
  org: OrganizationModel;
  isAuthenticated: boolean;
};

export function OrgProblemsTab({
  problems,
  pagination,
  org,
  isAuthenticated,
}: Props) {
  const [searchValue, setSearchValue] = useState("");
  const [createOpened, setCreateOpened] = useState(false);

  const filteredProblems = useMemo(
    () =>
      problems.filter((problem) =>
        problem.title?.toLowerCase().includes(searchValue.toLowerCase()),
      ),
    [problems, searchValue],
  );

  return (
    <Stack gap="md">
      <Group justify="space-between" align="center">
        <TextInput
          placeholder="Поиск задач..."
          leftSection={<IconSearch size={16} />}
          value={searchValue}
          onChange={(event) => setSearchValue(event.currentTarget.value)}
          radius="md"
          size="md"
          style={{ flex: 1 }}
        />
        {isAuthenticated && (
          <Button
            title="Создать новую задачу"
            onClick={() => setCreateOpened(true)}
            size="md"
            leftSection={<IconPlus size={18} />}
            radius="md"
          >
            Создать задачу
          </Button>
        )}
      </Group>

      {filteredProblems.length === 0 ? (
        <Center py="xl">
          <Text c="dimmed">Задачи не найдены</Text>
        </Center>
      ) : (
        <SimpleGrid
          cols={{ base: 1, xs: 2, sm: 2, md: 3, lg: 4, xl: 4 }}
          spacing={{ base: "xs", sm: "sm", md: "md" }}
        >
          {filteredProblems.map((problem) => (
            <ProblemCard
              key={problem.id}
              problem={problem}
              showEditButton={false}
            />
          ))}
        </SimpleGrid>
      )}

      {pagination.total > 1 && (
        <Stack align="center">
          <NextPagination
            pagination={pagination}
            baseUrl={`/orgs/${org.id}/problems`}
          />
        </Stack>
      )}

      <CreateProblemModal
        opened={createOpened}
        onClose={() => setCreateOpened(false)}
        orgs={[org]}
        defaultOrgId={org.id}
        lockOrganization
      />
    </Stack>
  );
}
