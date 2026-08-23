"use client";

import {
  ActionIcon,
  Badge,
  Card,
  Center,
  Group,
  Loader,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import {IconEdit, IconTrash} from "@tabler/icons-react";
import React, {type ReactNode} from "react";

export interface EntityColumn<T> {
  header: string;
  accessorKey?: keyof T;
  render?: (item: T) => ReactNode;
  width?: number | string;
  align?: "left" | "center" | "right";
}

export interface EntityManagementTableProps<T> {
  items: T[];
  loading?: boolean;
  emptyMessage?: string;
  emptyIcon?: ReactNode;
  getItemKey: (item: T) => string;
  columns?: EntityColumn<T>[];
  getTitle?: (item: T) => ReactNode;
  getSubtitle?: (item: T) => ReactNode;
  getRoleBadge?: (item: T) => {label: string; color: string} | null;
  onEdit?: (item: T) => void;
  onDelete?: (item: T) => void;
  deletingId?: string | null;
  canEdit?: (item: T) => boolean;
  canDelete?: (item: T) => boolean;
  editTooltip?: string;
  deleteTooltip?: string;
}

export const EntityManagementTable = <T,>({
  items,
  loading = false,
  emptyMessage = "Список пуст",
  emptyIcon,
  getItemKey,
  columns,
  getTitle,
  getSubtitle,
  getRoleBadge,
  onEdit,
  onDelete,
  deletingId,
  canEdit,
  canDelete,
  editTooltip: _editTooltip = "Изменить роль",
  deleteTooltip: _deleteTooltip = "Удалить",
}: EntityManagementTableProps<T>): ReactNode => {
  if (loading) {
    return (
      <Center py="xl">
        <Loader size="md" />
      </Center>
    );
  }

  if (items.length === 0) {
    return (
      <Center py="xl">
        <Stack align="center" gap="xs">
          {emptyIcon}
          <Text c="dimmed" size="sm">
            {emptyMessage}
          </Text>
        </Stack>
      </Center>
    );
  }

  const renderCellContent = (col: EntityColumn<T>, item: T): ReactNode => {
    if (col.render) {
      return col.render(item);
    }
    if (col.accessorKey) {
      return String(item[col.accessorKey] ?? "");
    }
    return null;
  };

  const hasActions = Boolean(onEdit || onDelete);

  return (
    <Card withBorder radius="md" p={0}>
      <Table.ScrollContainer minWidth={500}>
        <Table verticalSpacing="sm" horizontalSpacing="md" striped highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              {columns ? (
                columns.map((col, index) => (
                  <Table.Th key={index} style={{width: col.width, textAlign: col.align}}>
                    {col.header}
                  </Table.Th>
                ))
              ) : (
                <>
                  <Table.Th>Название / Пользователь</Table.Th>
                  {getRoleBadge && <Table.Th>Роль / Доступ</Table.Th>}
                </>
              )}
              {hasActions && <Table.Th style={{width: 100, textAlign: "right"}}>Действия</Table.Th>}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {items.map((item) => {
              const key = getItemKey(item);
              const isDeleting = deletingId === key;
              const allowEdit = canEdit ? canEdit(item) : Boolean(onEdit);
              const allowDelete = canDelete ? canDelete(item) : Boolean(onDelete);
              const roleBadge = getRoleBadge ? getRoleBadge(item) : null;

              return (
                <Table.Tr key={key}>
                  {columns ? (
                    columns.map((col, colIdx) => (
                      <Table.Td key={colIdx} style={{textAlign: col.align}}>
                        {renderCellContent(col, item)}
                      </Table.Td>
                    ))
                  ) : (
                    <>
                      <Table.Td>
                        <Stack gap={2}>
                          {getTitle ? (
                            <Text fw={500} size="sm">
                              {getTitle(item)}
                            </Text>
                          ) : null}
                          {getSubtitle ? (
                            <Text size="xs" c="dimmed">
                              {getSubtitle(item)}
                            </Text>
                          ) : null}
                        </Stack>
                      </Table.Td>
                      {getRoleBadge && (
                        <Table.Td>
                          {roleBadge && (
                            <Badge color={roleBadge.color} variant="light" size="sm">
                              {roleBadge.label}
                            </Badge>
                          )}
                        </Table.Td>
                      )}
                    </>
                  )}
                  {hasActions && (
                    <Table.Td style={{textAlign: "right"}}>
                      <Group gap="xs" justify="flex-end" wrap="nowrap">
                        {allowEdit && onEdit && (
                          <ActionIcon
                            variant="subtle"
                            color="gray"
                            size="sm"
                            onClick={() => onEdit(item)}
                          >
                            <IconEdit size={16} />
                          </ActionIcon>
                        )}
                        {allowDelete && onDelete && (
                          <ActionIcon
                            variant="subtle"
                            color="red"
                            size="sm"
                            loading={isDeleting}
                            onClick={() => onDelete(item)}
                          >
                            <IconTrash size={16} />
                          </ActionIcon>
                        )}
                      </Group>
                    </Table.Td>
                  )}
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      </Table.ScrollContainer>
    </Card>
  );
};
