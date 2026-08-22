"use client";

import {
  Badge,
  Button,
  Group,
  Paper,
  PasswordInput,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconLink} from "@tabler/icons-react";
import {useCallback, useEffect, useState} from "react";

import {api} from "@/lib/api";

import type {ClaimedAccountItem} from "@/contracts/core/v1";
import type {ReactNode} from "react";

export const ClaimTemporaryAccountSection = (): ReactNode => {
  const [loading, setLoading] = useState(false);
  const [claimedAccounts, setClaimedAccounts] = useState<ClaimedAccountItem[]>([]);
  const [fetchingList, setFetchingList] = useState(true);

  const form = useForm({
    initialValues: {
      username: "",
      password: "",
    },
    validate: {
      username: (value) => (!value.trim() ? "Введите логин временного аккаунта" : null),
      password: (value) => (!value ? "Введите пароль" : null),
    },
  });

  const loadClaimedAccounts = useCallback(async () => {
    setFetchingList(true);
    const [error, data] = await api.listMyClaimedAccounts();
    setFetchingList(false);
    if (!error && data?.accounts) {
      setClaimedAccounts(data.accounts);
    }
  }, []);

  useEffect(() => {
    loadClaimedAccounts();
  }, [loadClaimedAccounts]);

  const handleClaim = async (values: typeof form.values) => {
    setLoading(true);
    const [error, data] = await api.claimTemporaryUser({
      requestBody: {
        username: values.username.trim(),
        password: values.password,
      },
    });
    setLoading(false);

    if (error) {
      notifications.show({
        title: "Не удалось привязать аккаунт",
        message: error.message,
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Аккаунт успешно привязан",
      message: `Результаты аккаунта @${data?.claimed_username ?? values.username} привязаны к вашему профилю. Доступ к контестам обновлен.`,
      color: "green",
      icon: <IconCheck size={16} />,
    });

    form.reset();
    await loadClaimedAccounts();
  };

  return (
    <Paper shadow="sm" p="lg" radius="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <div>
            <Title order={3} size="h4">
              Привязка результатов олимпиад
            </Title>
            <Text size="sm" c="dimmed" mt={4}>
              Свяжите временный аккаунт участника с вашим постоянным профилем для доступа к контестам и
              дорешиванию.
            </Text>
          </div>
        </Group>

        <form onSubmit={form.onSubmit(handleClaim)}>
          <Group align="flex-end" gap="md">
            <TextInput
              label="Логин временного аккаунта"
              placeholder="school_olymp_01"
              required
              style={{flex: 1}}
              {...form.getInputProps("username")}
            />
            <PasswordInput
              label="Пароль"
              placeholder="Пароль от временного аккаунта"
              required
              style={{flex: 1}}
              {...form.getInputProps("password")}
            />
            <Button
              type="submit"
              loading={loading}
              leftSection={<IconLink size={16} />}
            >
              Привязать
            </Button>
          </Group>
        </form>

        {claimedAccounts.length > 0 && (
          <Stack gap="xs" mt="sm">
            <Text size="sm" fw={600}>
              Привязанные олимпиадные аккаунты:
            </Text>
            <Table withTableBorder highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Логин</Table.Th>
                  <Table.Th>Дата привязки</Table.Th>
                  <Table.Th>Срок действия</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {claimedAccounts.map((account) => (
                  <Table.Tr key={account.id}>
                    <Table.Td>
                      <Group gap="xs">
                        <Text fw={500} size="sm">
                          @{account.username}
                        </Text>
                        <Badge size="xs" color="teal" variant="light">
                          Привязан
                        </Badge>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" c="dimmed">
                        {new Date(account.claimed_at).toLocaleDateString("ru-RU", {
                          day: "2-digit",
                          month: "long",
                          year: "numeric",
                        })}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" c="dimmed">
                        {account.expires_at
                          ? new Date(account.expires_at).toLocaleDateString("ru-RU")
                          : "Бессрочно"}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        )}

        {!fetchingList && claimedAccounts.length === 0 && (
          <Text size="xs" c="dimmed" fs="italic">
            У вас пока нет привязанных временных аккаунтов.
          </Text>
        )}
      </Stack>
    </Paper>
  );
};
