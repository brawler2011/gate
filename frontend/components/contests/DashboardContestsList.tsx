"use client";

import { Group, Stack, Text, Title, Badge, Card, ThemeIcon } from "@mantine/core";
import { IconTrophy, IconChevronRight } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import type { DashboardContestModel } from "@contracts/core/v1";

type DashboardContestsListProps = {
  contests: DashboardContestModel[];
};

export function DashboardContestsList({ contests }: DashboardContestsListProps) {
  const router = useRouter();

  if (contests.length === 0) {
    return (
      <Card withBorder radius="md" p="xl" style={{ textAlign: "center" }}>
        <Text c="dimmed" size="sm">
          Вы пока не принимали участия в контестах
        </Text>
      </Card>
    );
  }

  const getContestStatus = (start?: string | null, end?: string | null) => {
    if (!start) return { label: "Идет", color: "green" };
    const now = new Date();
    const startTime = new Date(start);
    const endTime = end ? new Date(end) : null;

    if (now < startTime) {
      return { label: "Скоро", color: "yellow" };
    }
    if (endTime && now > endTime) {
      return { label: "Завершен", color: "gray" };
    }
    return { label: "Идет", color: "green" };
  };

  const getRoleBadge = (role: string) => {
    switch (role) {
      case "owner":
        return { label: "Создатель", color: "violet" };
      case "moderator":
        return { label: "Модератор", color: "teal" };
      default:
        return { label: "Участник", color: "blue" };
    }
  };

  return (
    <Stack gap="sm">
      {contests.map((contest) => {
        const status = getContestStatus(contest.start_time, contest.end_time);
        const role = getRoleBadge(contest.user_role);

        return (
          <Card
            key={contest.id}
            withBorder
            radius="md"
            p="md"
            style={{
              cursor: "pointer",
              transition: "transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease",
            }}
            onClick={() => router.push(`/contests/${contest.id}`)}
            onMouseEnter={(e) => {
              const el = e.currentTarget;
              el.style.transform = "translateY(-2px)";
              el.style.boxShadow = "var(--mantine-shadow-md)";
              el.style.borderColor = "var(--mantine-color-blue-light-color)";
            }}
            onMouseLeave={(e) => {
              const el = e.currentTarget;
              el.style.transform = "none";
              el.style.boxShadow = "none";
              el.style.borderColor = "";
            }}
          >
            <Group justify="space-between" align="center" wrap="nowrap">
              <Group gap="md" style={{ flex: 1, minWidth: 0 }}>
                <ThemeIcon
                  variant="light"
                  size="xl"
                  radius="md"
                  color={status.color}
                >
                  <IconTrophy size={20} />
                </ThemeIcon>

                <Stack gap={2} style={{ flex: 1, minWidth: 0 }}>
                  <Text size="xs" c="dimmed" fw={500} lineClamp={1}>
                    {contest.organization_name}
                  </Text>
                  <Title order={4} size="md" fw={600} lineClamp={1}>
                    {contest.title}
                  </Title>
                </Stack>
              </Group>

              <Group gap="xs" style={{ flexShrink: 0 }}>
                <Badge size="sm" variant="light" color={role.color}>
                  {role.label}
                </Badge>
                <Badge size="sm" variant="filled" color={status.color}>
                  {status.label}
                </Badge>
                <IconChevronRight size={18} style={{ opacity: 0.3 }} />
              </Group>
            </Group>
          </Card>
        );
      })}
    </Stack>
  );
}
