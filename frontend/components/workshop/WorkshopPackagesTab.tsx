"use client";

import {
  Badge,
  Button,
  Group,
  Loader,
  ScrollArea,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useCallback, useEffect, useState, useTransition } from "react";
import { SectionPaper } from "@/components/workshop/SectionPaper";
import { listProblemPackages, publishProblem } from "@/lib/actions";

type PackageItem = {
  id?: string;
  version?: number;
  status?: string;
  git_commit_hash?: string;
  created_at?: string;
  compiled_at?: string;
};

type Props = {
  problemId: string;
};

function StatusBadge({ status }: { status?: string }) {
  const map: Record<string, { color: string; label: string }> = {
    ready: { color: "green", label: "Готов" },
    building: { color: "blue", label: "Сборка" },
    pending: { color: "gray", label: "Ожидание" },
    failed: { color: "red", label: "Ошибка" },
  };
  const info = map[status ?? ""] ?? { color: "gray", label: status ?? "—" };
  return <Badge color={info.color} variant="light">{info.label}</Badge>;
}

function formatDateTime(iso?: string) {
  if (!iso) return "—";
  try {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return "—";
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  } catch {
    return "—";
  }
}

export function WorkshopPackagesTab({ problemId }: Props) {
  const [isBuilding, startBuilding] = useTransition();
  const [packages, setPackages] = useState<PackageItem[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchPackages = useCallback(async () => {
    setLoading(true);
    const [error, data] = await listProblemPackages(problemId);
    if (!error && data?.packages) {
      setPackages(data.packages as PackageItem[]);
    }
    setLoading(false);
  }, [problemId]);

  useEffect(() => {
    fetchPackages();
  }, [fetchPackages]);

  const handleBuild = () => {
    startBuilding(async () => {
      const [error, data] = await publishProblem(problemId);
      if (error) {
        notifications.show({
          title: "Ошибка сборки пакета",
          message: error.message ?? "Не удалось собрать пакет",
          color: "red",
        });
        return;
      }
      notifications.show({
        title: "Пакет собран",
        message: data?.version != null ? `Версия пакета: v${data.version}` : "Пакет успешно собран",
        color: "green",
      });
      await fetchPackages();
    });
  };

  return (
    <ScrollArea style={{ flex: 1 }} p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Сборка пакета">
          <Stack gap="sm">
            <Text size="sm" c="dimmed">
              Соберите пакет задачи из текущего состояния воркшопа. После успешной сборки задача
              будет готова к тестированию решений на всех контестах, где она добавлена.
            </Text>
            <Group>
              <Button loading={isBuilding} onClick={handleBuild}>
                Сбилдить пакет
              </Button>
            </Group>
          </Stack>
        </SectionPaper>

        <SectionPaper title="История сборок">
          {loading ? (
            <Group justify="center" py="md">
              <Loader size="sm" />
            </Group>
          ) : packages.length === 0 ? (
            <Text size="sm" c="dimmed">
              Пакеты ещё не собирались.
            </Text>
          ) : (
            <Table striped highlightOnHover withTableBorder withColumnBorders>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Версия</Table.Th>
                  <Table.Th>Статус</Table.Th>
                  <Table.Th>Коммит</Table.Th>
                  <Table.Th>Дата сборки</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {packages.map((pkg) => (
                  <Table.Tr key={pkg.id}>
                    <Table.Td>
                      <Text size="sm" fw={600}>v{pkg.version}</Text>
                    </Table.Td>
                    <Table.Td>
                      <StatusBadge status={pkg.status} />
                    </Table.Td>
                    <Table.Td>
                      <Text size="xs" ff="monospace" c="dimmed">
                        {pkg.git_commit_hash?.slice(0, 8) ?? "—"}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{formatDateTime(pkg.created_at)}</Text>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
        </SectionPaper>
      </Stack>
    </ScrollArea>
  );
}
