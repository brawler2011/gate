"use client";

import { Group, Stack, Text, Title, Card, ThemeIcon, ActionIcon } from "@mantine/core";
import { IconPencil, IconCode } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import type { DashboardProblemModel } from "@contracts/core/v1";
import { APP_COLORS } from "@/lib/theme/colors";

type DashboardProblemsListProps = {
  problems: DashboardProblemModel[];
};

export function DashboardProblemsList({ problems }: DashboardProblemsListProps) {
  const router = useRouter();

  if (problems.length === 0) {
    return (
      <Card withBorder radius="md" p="xl" style={{ textAlign: "center" }}>
        <Text c="dimmed" size="sm">
          У вас пока нет созданных или редактируемых задач
        </Text>
      </Card>
    );
  }

  const formatTimeLimit = (timeMs: number) => {
    if (timeMs % 1000 === 0) {
      return `${timeMs / 1000}с`;
    }
    return `${timeMs}мс`;
  };

  return (
    <Stack gap="sm">
      {problems.map((problem) => (
        <Card
          key={problem.id}
          withBorder
          radius="md"
          p="md"
          style={{
            cursor: "pointer",
            transition: "transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease",
          }}
          onClick={() => router.push(`/problems/${problem.id}`)}
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
                color="blue"
              >
                <IconCode size={20} />
              </ThemeIcon>

              <Stack gap={2} style={{ flex: 1, minWidth: 0 }}>
                <Text size="xs" c="dimmed" fw={500} lineClamp={1}>
                  {problem.organization_name}
                </Text>
                <Title order={4} size="md" fw={600} lineClamp={1}>
                  {problem.title}
                </Title>
                <Group gap="xs" mt={2}>
                  <Text size="xs" c="dimmed">
                    ⏱️ {formatTimeLimit(problem.time_limit)}
                  </Text>
                  <Text size="xs" c="dimmed">|</Text>
                  <Text size="xs" c="dimmed">
                    💾 {problem.memory_limit} MB
                  </Text>
                </Group>
              </Stack>
            </Group>

            <ActionIcon
              variant="subtle"
              size="lg"
              radius="md"
              color={APP_COLORS.problems}
              component="div"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                router.push(`/problems/${problem.id}`);
              }}
              style={{ flexShrink: 0 }}
            >
              <IconPencil size={20} />
            </ActionIcon>
          </Group>
        </Card>
      ))}
    </Stack>
  );
}
