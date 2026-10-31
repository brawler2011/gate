import {Button, Center, Group, Input, Stack, Text} from "@mantine/core";
import {IconPlus, IconSearch} from "@tabler/icons-react";
import {useCallback, useEffect, useRef, useState} from "react";
import {useNavigate, useSearchParams} from "react-router-dom";

import {ContestsTable} from "@/components/contests/ContestsTable";
import {OrgJoinButton} from "@/components/orgs/OrgJoinButton";
import {RoutePagination} from "@/components/shared/Pagination";
import {CreateContestModal} from "@/components/workshop/CreateContestModal";

import type {ContestModel, OrganizationModel, PaginationModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type Props = {
  contests: ContestModel[];
  pagination: PaginationModel;
  org: OrganizationModel;
  isAuthenticated: boolean;
  isMember: boolean;
  search?: string;
};

export const OrgContestsTab = ({
  contests,
  pagination,
  org,
  isAuthenticated,
  isMember,
  search = "",
}: Props): ReactNode => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const searchTimeoutRef = useRef<NodeJS.Timeout | undefined>(undefined);
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
      params.delete("tab");
      params.delete("page");
      if (nextSearch) {
        params.set("search", nextSearch);
      } else {
        params.delete("search");
      }

      navigate(`/${org.login}?${params.toString()}`);
    },
    [org.login, navigate, searchParams],
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
          style={{flex: 1}}
        />
        <Group gap="sm">
          <OrgJoinButton org={org} isAuthenticated={isAuthenticated} isMember={isMember} />
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
          <RoutePagination
            pagination={pagination}
            baseUrl={`/${org.login}`}
            queryParams={{search}}
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
};
