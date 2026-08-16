"use client";

import {
  Badge,
  Card,
  Container,
  Group,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {
  IconActivity,
  IconBuilding,
  IconCheck,
  IconFileCode,
  IconFileText,
  IconNews,
  IconPuzzle,
  IconTrophy,
  IconUsers,
} from "@tabler/icons-react";
import Link from "next/link";
import useSWR from "swr";

import {api} from "@/lib/api";

import type {ReactNode} from "react";

export const AdminDashboardContent = (): ReactNode => {
  // Fetch overview counts
  const {data: statsData, isLoading: isStatsLoading} = useSWR(
    "admin-dashboard-stats",
    async () => {
      const [
        [usersErr, usersRes],
        [contestsErr, contestsRes],
        [problemsErr, problemsRes],
        [submissionsErr, submissionsRes],
        [healthErr, healthRes],
      ] = await Promise.all([
        api.listUsers({page: 1, pageSize: 1}),
        api.listAdminContests({page: 1, pageSize: 1}),
        api.listProblems({page: 1, pageSize: 1}),
        api.listSubmissions({page: 1, pageSize: 1}),
        api.getHealth(),
      ]);

      return {
        usersCount: usersRes?.pagination?.total ?? 0,
        contestsCount: contestsRes?.pagination?.total ?? 0,
        problemsCount: problemsRes?.pagination?.total ?? 0,
        submissionsCount: submissionsRes?.pagination?.total ?? 0,
        healthStatus: healthErr ? "error" : healthRes?.status || "ok",
        healthMessage: healthErr ? "Ошибка подключения к бэкенду" : healthRes?.message || "Бэкенд работает нормально",
      };
    }
  );

  const stats = [
    {
      title: "Пользователи",
      value: statsData?.usersCount,
      icon: IconUsers,
      color: "blue",
      href: "/admin/users",
      description: "Всего зарегистрированных аккаунтов",
    },
    {
      title: "Контесты",
      value: statsData?.contestsCount,
      icon: IconTrophy,
      color: "teal",
      href: "/admin/contests",
      description: "Соревнования на платформе",
    },
    {
      title: "Задачи",
      value: statsData?.problemsCount,
      icon: IconPuzzle,
      color: "violet",
      href: "/admin/problems",
      description: "Задачи в базе платформы",
    },
    {
      title: "Посылки",
      value: statsData?.submissionsCount,
      icon: IconFileCode,
      color: "orange",
      href: "/admin/submissions",
      description: "Отправленные решения пользователей",
    },
  ];

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
        {/* Header Title */}
        <div>
          <Title order={2}>Административная панель</Title>
          <Text c="dimmed" size="sm">
            Обзор метрик системы и быстрое управление разделами
          </Text>
        </div>

        {/* System Health Card */}
        <Paper withBorder p="md" radius="md">
          <Group justify="space-between">
            <Group gap="sm">
              <ThemeIcon
                size="lg"
                radius="md"
                color={statsData?.healthStatus === "error" ? "red" : "green"}
                variant="light"
              >
                <IconActivity size={20} />
              </ThemeIcon>
              <div>
                <Text fw={600} size="sm">
                  Статус бэкенда и сервисов
                </Text>
                <Text size="xs" c="dimmed">
                  {statsData?.healthMessage || "Загрузка статуса..."}
                </Text>
              </div>
            </Group>
            {isStatsLoading ? (
              <Skeleton height={24} width={80} radius="xl" />
            ) : (
              <Badge
                color={statsData?.healthStatus === "error" ? "red" : "green"}
                variant="light"
                size="md"
                leftSection={<IconCheck size={12} />}
              >
                {statsData?.healthStatus === "error" ? "Ошибка" : "Онлайн"}
              </Badge>
            )}
          </Group>
        </Paper>

        {/* Metric Stat Cards */}
        <SimpleGrid cols={{base: 1, sm: 2, md: 4}} spacing="md">
          {stats.map((stat) => {
            const Icon = stat.icon;
            return (
              <Paper
                key={stat.title}
                withBorder
                p="md"
                radius="md"
                component={Link}
                href={stat.href}
                style={{textDecoration: "none", color: "inherit"}}
              >
                <Group justify="space-between" mb="xs">
                  <Text size="xs" c="dimmed" fw={700} tt="uppercase">
                    {stat.title}
                  </Text>
                  <ThemeIcon color={stat.color} variant="light" size="md" radius="md">
                    <Icon size={18} />
                  </ThemeIcon>
                </Group>
                {isStatsLoading ? (
                  <Skeleton height={32} width={100} mb={4} radius="sm" />
                ) : (
                  <Text fw={700} size="xl" lh={1}>
                    {stat.value ?? 0}
                  </Text>
                )}
                <Text size="xs" c="dimmed" mt={7}>
                  {stat.description}
                </Text>
              </Paper>
            );
          })}
        </SimpleGrid>

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
