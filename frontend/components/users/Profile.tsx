"use client";

import {
  Anchor,
  Avatar,
  Badge,
  Button,
  Center,
  Container,
  Group,
  Modal,
  Paper,
  Stack,
  Table,
  Tabs,
  Text,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";
import { NextPagination } from '@/components/shared/Pagination';
import { useDisclosure } from "@mantine/hooks";
import { useForm } from "@mantine/form";
import { IconCalendar, IconEdit, IconTrophy } from "@tabler/icons-react";
import { getRoleColor, TimeBeautify } from "@/lib/lib";
import { APP_COLORS } from "@/lib/theme/colors";
import { patchMe } from "@/lib/actions";
import type { ContestModel } from "@contracts/core/v1";
import Link from "next/link";
import { useRouter } from "next/navigation";

type ProfileProps = {
  username: string;
  name?: string;
  surname?: string;
  role: string;
  bio?: string;
  createdAt?: string;
  userId: string;
  contests?: ContestModel[];
  contestsPagination?: { page: number; total: number };
  contestsPage?: number;
  isOwnProfile?: boolean;
};

const Profile = (props: ProfileProps) => {
  const showRole = props.role?.toLowerCase() !== "user";
  const fullName = [props.name, props.surname].filter(Boolean).join(" ");
  const initials = props.username?.[0]?.toUpperCase() ?? "?";
  const contests = props.contests ?? [];

  const [opened, { open, close }] = useDisclosure(false);
  const router = useRouter();

  const form = useForm({
    initialValues: {
      name: props.name ?? "",
      surname: props.surname ?? "",
      bio: props.bio ?? "",
    },
  });

  const handleSubmit = async (values: typeof form.values) => {
    await patchMe({
      name: values.name || undefined,
      surname: values.surname || undefined,
      bio: values.bio || undefined,
    });
    close();
    router.refresh();
  };

  return (
    <Container size="md" px={0}>
      <Stack gap="lg">
        {/* Header card */}
        <Paper shadow="sm" p="lg" radius="md">
          <Group align="flex-start" gap="xl">
            <Avatar size={72} radius="xl" color={APP_COLORS.users}>
              {initials}
            </Avatar>
            <Stack gap="xs" style={{ flex: 1 }}>
              <Group gap="sm" align="center" justify="space-between">
                <Group gap="sm" align="center">
                  <Title order={2}>@{props.username}</Title>
                  {showRole && (
                    <Badge color={getRoleColor(props.role)} size="lg">
                      {props.role}
                    </Badge>
                  )}
                </Group>
                {props.isOwnProfile && (
                  <Button
                    variant="filled"
                    size="xs"
                    leftSection={<IconEdit size={14} />}
                    onClick={open}
                  >
                    Редактировать
                  </Button>
                )}
              </Group>
              {fullName && (
                <Text size="lg" fw={500}>
                  {fullName}
                </Text>
              )}
              {props.bio && (
                <Text c="dimmed" size="sm">
                  {props.bio}
                </Text>
              )}
              <Group gap="lg">
                {props.createdAt && (
                  <Group gap="xs">
                    <IconCalendar size={14} style={{ color: "var(--mantine-color-dimmed)" }} />
                    <Text size="sm" c="dimmed">
                      На платформе с {new Date(props.createdAt!).toLocaleDateString("ru-RU", { day: "2-digit", month: "long", year: "numeric" })}
                    </Text>
                  </Group>
                )}
              </Group>
            </Stack>
          </Group>
        </Paper>

        {/* Contests tabs */}
        <Tabs defaultValue="contests">
          <Tabs.List>
            <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
              Контесты
            </Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="contests" pt="md">
            {contests.length > 0 ? (
              <>
                <Table striped highlightOnHover>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Название</Table.Th>
                      <Table.Th>Видимость</Table.Th>
                      <Table.Th>Дата создания</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {contests.map((contest) => (
                      <Table.Tr key={contest.id}>
                        <Table.Td>
                          <Anchor component={Link} href={`/contests/${contest.id}`} size="sm">
                            {contest.title}
                          </Anchor>
                        </Table.Td>
                        <Table.Td>
                          <Badge
                            color={contest.visibility === "public" ? "green" : "gray"}
                            variant="light"
                            size="sm"
                          >
                            {contest.visibility}
                          </Badge>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" c="dimmed">
                            {TimeBeautify(contest.created_at)}
                          </Text>
                        </Table.Td>
                      </Table.Tr>
                    ))}
                  </Table.Tbody>
                </Table>
                {props.contestsPagination && props.contestsPagination.total > 1 && (
                  <Stack align="center" mt="md">
                    <NextPagination
                      pagination={{ page: props.contestsPage ?? 1, total: props.contestsPagination.total }}
                      baseUrl={`/users/${props.userId}`}
                      queryParams={{ contestsPage: props.contestsPage }}
                    />
                  </Stack>
                )}
              </>
            ) : (
              <Center py="xl">
                <Stack align="center" gap="sm">
                  <IconTrophy size={40} style={{ opacity: 0.3 }} />
                  <Text c="dimmed">Нет публичных контестов</Text>
                </Stack>
              </Center>
            )}
          </Tabs.Panel>
        </Tabs>

        {/* Edit profile modal */}
        <Modal opened={opened} onClose={close} title="Редактировать профиль">
          <form onSubmit={form.onSubmit(handleSubmit)}>
            <Stack>
              <TextInput label="Имя" placeholder="Иван" {...form.getInputProps("name")} />
              <TextInput label="Фамилия" placeholder="Иванов" {...form.getInputProps("surname")} />
              <Textarea
                label="О себе"
                placeholder="Расскажите о себе..."
                autosize
                minRows={3}
                {...form.getInputProps("bio")}
              />
              <Button type="submit">Сохранить</Button>
            </Stack>
          </form>
        </Modal>
      </Stack>
    </Container>
  );
};

export { Profile };
