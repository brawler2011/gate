"use client";

import {
  Card,
  Container,
  Group,
  SimpleGrid,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {
  IconBuilding,
  IconFileCode,
  IconNews,
  IconPuzzle,
  IconTrophy,
  IconUsers,
} from "@tabler/icons-react";
import Link from "next/link";

import type {ReactNode} from "react";

export const AdminDashboardContent = (): ReactNode => {
  const quickNav = [
    {
      title: "Управление пользователями",
      desc: "Просмотр списка пользователей, смена ролей, переход к посылкам",
      href: "/admin/users",
      icon: IconUsers,
      color: "blue",
    },
    {
      title: "Управление контестами",
      desc: "Список контестов, редактирование, удаление, просмотр результатов",
      href: "/admin/contests",
      icon: IconTrophy,
      color: "teal",
    },
    {
      title: "База задач",
      desc: "Управление задачами, проверка ограничений, удаление",
      href: "/admin/problems",
      icon: IconPuzzle,
      color: "violet",
    },
    {
      title: "Мониторинг посылок",
      desc: "Глобальная таблица решений, фильтрация по вердиктам, ретест",
      href: "/admin/submissions",
      icon: IconFileCode,
      color: "orange",
    },
    {
      title: "Публикации и Блоги",
      desc: "Модерация постов и новостных публикаций",
      href: "/admin/blogs",
      icon: IconNews,
      color: "cyan",
    },
    {
      title: "Организации",
      desc: "Управление учебными заведениями и организациями",
      href: "/admin/orgs",
      icon: IconBuilding,
      color: "indigo",
    },
  ];

  return (
    <Container size="xl" py="md">
      <Stack gap="lg">
        {/* Quick Section Navigation Grid */}
        <div>
          <Title order={4} mb="md">
            Разделы управления
          </Title>
          <SimpleGrid cols={{base: 1, sm: 2, md: 3}} spacing="md">
            {quickNav.map((nav) => {
              const Icon = nav.icon;
              return (
                <Card
                  key={nav.title}
                  withBorder
                  padding="md"
                  radius="md"
                  component={Link}
                  href={nav.href}
                  style={{textDecoration: "none"}}
                >
                  <Group gap="sm" mb="xs">
                    <ThemeIcon color={nav.color} variant="light" size="lg" radius="md">
                      <Icon size={20} />
                    </ThemeIcon>
                    <Text fw={600} size="md">
                      {nav.title}
                    </Text>
                  </Group>
                  <Text size="sm" c="dimmed">
                    {nav.desc}
                  </Text>
                </Card>
              );
            })}
          </SimpleGrid>
        </div>
      </Stack>
    </Container>
  );
};
