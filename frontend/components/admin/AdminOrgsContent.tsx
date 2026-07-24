"use client";

import { deleteOrganization } from "@/lib/actions";
import {
  ActionIcon,
  Box,
  Center,
  Container,
  Group,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconSearch, IconTrash } from "@tabler/icons-react";
import { useEffect, useRef, useState } from "react";
import type { OrganizationModel } from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { TruncatedWithCopy } from '@/components/shared/TruncatedWithCopy';
import { DeleteOrgModal } from "./DeleteOrgModal";
import { CreateOrgButton } from "../orgs/CreateOrgButton";
import useSWR from "swr";
import { useRouter, useSearchParams } from "next/navigation";
import classes from "./AdminPage.module.css";

type AdminOrgsContentProps = {
  page: number;
  search?: string;
};

export function AdminOrgsContent({ page, search }: AdminOrgsContentProps) {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Search input state
  const [searchInput, setSearchInput] = useState(search || "");
  const searchTimeoutRef = useRef<NodeJS.Timeout>();
  const isFirstRender = useRef(true);

  // Delete modal state
  const [deleteModalOpened, setDeleteModalOpened] = useState(false);
  const [orgToDelete, setOrgToDelete] = useState<OrganizationModel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Sync state with URL when searchParams change externally
  useEffect(() => {
    setSearchInput(search || "");
  }, [search]);

  // Handle search input change with debounce
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      const params = new URLSearchParams(searchParams);
      params.delete("page"); // Reset to first page
      if (searchInput) {
        params.set("search", searchInput);
      } else {
        params.delete("search");
      }
      router.push(`/admin/orgs?${params.toString()}`);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchInput, router, searchParams]);

  // Fetch organizations
  const { data, error, isLoading, mutate } = useSWR(
    `/api/organizations?page=${page}&pageSize=10${search ? `&search=${encodeURIComponent(search)}` : ""}`,
    async (url) => {
      const res = await fetch(url);
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.message || "Не удалось загрузить организации");
      }
      return res.json();
    }
  );

  const orgs = data?.organizations || [];
  const pagination = data?.pagination || { total: 0, page: page };

  const handleDeleteClick = (e: React.MouseEvent, org: OrganizationModel) => {
    e.stopPropagation();
    setOrgToDelete(org);
    setDeleteModalOpened(true);
  };

  const handleDeleteConfirm = async () => {
    if (!orgToDelete) return;

    setDeletingId(orgToDelete.id);
    try {
      const [err] = await deleteOrganization(orgToDelete.id);
      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось удалить организацию",
          color: "red",
        });
        throw new Error(err.message);
      }

      notifications.show({
        title: "Успех",
        message: "Организация успешно удалена",
        color: "green",
      });
      mutate();
    } finally {
      setDeletingId(null);
      setOrgToDelete(null);
    }
  };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке организаций</Title>
            <Text c="dimmed">{error.message}</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;
  const queryParams: Record<string, string | number | undefined> = {};
  if (search) queryParams.search = search;

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={3}>Организации</Title>
          <CreateOrgButton />
        </Group>

        <TextInput
          placeholder="Поиск организаций..."
          leftSection={<IconSearch size={16} />}
          value={searchInput}
          onChange={(e) => setSearchInput(e.currentTarget.value)}
          style={{ maxWidth: 400 }}
        />

        {isLoading ? (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        ) : orgs.length === 0 ? (
          <Center py="xl">
            <Text c="dimmed">Организации не найдены</Text>
          </Center>
        ) : (
          <>
            <Box className={classes.tableContainer}>
              <Table className={classes.table} verticalSpacing="xs">
                <Table.Thead className={classes.thead}>
                  <Table.Tr>
                    <Table.Th style={{ width: "30%" }}>Название</Table.Th>
                    <Table.Th style={{ width: "25%" }}>ID</Table.Th>
                    <Table.Th style={{ width: "25%" }}>Описание</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Создана</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Действия</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody className={classes.tbody}>
                  {orgs.map((org: OrganizationModel) => (
                    <Table.Tr
                      key={org.id}
                      onClick={() => router.push(`/orgs/${org.id}`)}
                    >
                      <Table.Td>
                        <Text className={classes.titleCell} lineClamp={1}>
                          {org.name}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <TruncatedWithCopy value={org.id} />
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.metaCell} lineClamp={1}>
                          {org.description || "—"}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.dateCell}>
                          {new Date(org.created_at).toLocaleDateString("ru-RU")}
                        </Text>
                      </Table.Td>
                      <Table.Td className={classes.actionsCell}>
                        <Group gap="xs" wrap="nowrap">
                          <ActionIcon
                            color="red"
                            variant="subtle"
                            onClick={(e) => handleDeleteClick(e, org)}
                            loading={deletingId === org.id}
                          >
                            <IconTrash size={16} />
                          </ActionIcon>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Box>

            {totalPages > 1 && (
              <Stack align="center" gap="md">
                <NextPagination
                  pagination={{
                    page: page,
                    total: totalPages,
                  }}
                  baseUrl="/admin/orgs"
                  queryParams={queryParams}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      {orgToDelete && (
        <DeleteOrgModal
          opened={deleteModalOpened}
          onClose={() => {
            setDeleteModalOpened(false);
            setOrgToDelete(null);
          }}
          org={{
            id: orgToDelete.id,
            name: orgToDelete.name,
          }}
          onSubmit={handleDeleteConfirm}
        />
      )}
    </Container>
  );
}
