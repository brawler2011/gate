"use client";

import { ContestsTable } from "@/components/contests/ContestsTable";
import { NextPagination } from "@/components/shared/Pagination";
import { CreateContestModal } from "@/components/workshop/CreateContestModal";
import type {
  ContestModel,
  OrganizationModel,
  PaginationModel,
} from "@contracts/gateway/v1";
import { Button, Center, Group, Input, Stack, Text } from "@mantine/core";
import { IconPlus, IconSearch } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

type Props = {
  contests: ContestModel[];
  pagination: PaginationModel;
  org: OrganizationModel;
  isAuthenticated: boolean;
  search?: string;
};

export function OrgContestsTab({
  contests,
  pagination,
  org,
  isAuthenticated,
  search = "",
}: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const searchTimeoutRef = useRef<NodeJS.Timeout>();
  const [searchValue, setSearchValue] = useState(search);
  const [createOpened, setCreateOpened] = useState(false);

  useEffect(() => {
    setSearchValue(search);
  }, [search]);

  const updateURL = useCallback(
    (nextSearch: string) => {
      const currentSearch = searchParams.get("search") || "";
      if (nextSearch === currentSearch) {
        return;
      }

      const params = new URLSearchParams(searchParams.toString());
      params.set("tab", "contests");
      params.delete("page");
      if (nextSearch) {
        params.set("search", nextSearch);
      } else {
        params.delete("search");
      }

      router.push(`/orgs/${org.id}?${params.toString()}`);
    },
    [org.id, router, searchParams],
  );

  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      updateURL(searchValue);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchValue, updateURL]);

  return (
    <Stack gap="md">
      <Group justify="space-between" align="center">
        <Input
          placeholder="Поиск контестов..."
          leftSection={<IconSearch size={16} />}
          value={searchValue}
          onChange={(event) => setSearchValue(event.currentTarget.value)}
          radius="md"
          size="md"
          style={{ flex: 1 }}
        />
        {isAuthenticated && (
          <Button
            title="Создать новый контест"
            onClick={() => setCreateOpened(true)}
            size="md"
            leftSection={<IconPlus size={18} />}
            radius="md"
          >
            Создать контест
          </Button>
        )}
      </Group>

      {contests.length === 0 ? (
        <Center py="xl">
          <Text c="dimmed">
            {search
              ? "Контесты по вашему запросу не найдены"
              : "В организации пока нет контестов"}
          </Text>
        </Center>
      ) : (
        <ContestsTable contests={contests} />
      )}

      {pagination.total > 1 && (
        <Center>
          <NextPagination
            pagination={pagination}
            baseUrl={`/orgs/${org.id}`}
            queryParams={{ tab: "contests", search }}
          />
        </Center>
      )}

      <CreateContestModal
        opened={createOpened}
        onClose={() => setCreateOpened(false)}
        orgs={[org]}
        defaultOrgId={org.id}
        lockOrganization
      />
    </Stack>
  );
}
