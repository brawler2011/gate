"use client";

import {Alert, Anchor, Badge, Box, Button, Code, Group, List, Loader, Pagination, Stack, Table, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {usePagedQuery} from "@/hooks/usePagedQuery";
import {api, unwrap} from "@/lib/api";

import {buildProblems} from "./draft/draftOps";
import {useProblemDraft} from "./draft/useDraft";

import type {ProblemBuild} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type Props = {
  problemId: string;
};

const STATE_BADGES: Record<ProblemBuild["state"], {color: string; label: string}> = {
  queued: {color: "gray", label: "В очереди"},
  running: {color: "blue", label: "Сборка"},
  succeeded: {color: "green", label: "Готово"},
  failed: {color: "red", label: "Ошибка"},
  cancelled: {color: "yellow", label: "Отменена"},
};

const ACTIVE_STATES = new Set<ProblemBuild["state"]>(["queued", "running"]);

const formatDateTime = (iso: string | null): string =>
  iso ? new Date(iso).toLocaleString("ru-RU", {day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit"}) : "—";

type Diagnostic = {stage?: string; test?: number; kind?: string; message?: string; truncated?: boolean};

const DiagnosticView = ({diagnostic}: {diagnostic: ProblemBuild["diagnostic"]}): ReactNode => {
  if (!diagnostic) {
    return null;
  }
  const info = diagnostic as Diagnostic;
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed">
        {info.stage ?? "сборка"}
        {info.test ? `, тест ${info.test}` : ""}
        {info.kind ? ` · ${info.kind}` : ""}
      </Text>
      {info.message && (
        <Code block style={{maxHeight: 160, overflow: "auto", whiteSpace: "pre-wrap"}}>
          {info.message}
          {info.truncated ? "\n… (вывод обрезан)" : ""}
        </Code>
      )}
    </Stack>
  );
};

export const WorkshopPackagesTab = ({problemId}: Props): ReactNode => {
  const queryClient = useQueryClient();
  const draft = useProblemDraft(problemId);
  const problems = draft.data ? buildProblems(draft.data) : [];

  const builds = useQuery({
    queryKey: ["problem-builds", problemId],
    queryFn: () => unwrap(api.GET("/problems/{id}/builds", {params: {path: {id: problemId}}})),
    // Poll while any build is still in flight.
    refetchInterval: (query) => (query.state.data?.some((build) => ACTIVE_STATES.has(build.state)) ? 2000 : false),
  });

  const packages = usePagedQuery(["problem-packages", problemId], (page, pageSize) =>
    unwrap(api.GET("/problems/{id}/packages", {params: {path: {id: problemId}, query: {page, pageSize}}})),
  );

  const refresh = () => {
    void queryClient.invalidateQueries({queryKey: ["problem-builds", problemId]});
    void queryClient.invalidateQueries({queryKey: ["problem-packages", problemId]});
  };

  const build = useMutation({
    mutationFn: async () => {
      const current = draft.data;
      if (!current) {
        throw new Error("Черновик ещё не загружен");
      }
      return unwrap(
        api.POST("/problems/{id}/packages/build", {
          params: {path: {id: problemId}, header: {"Idempotency-Key": crypto.randomUUID()}},
          body: {expected_revision: current.revision},
        }),
      );
    },
    onSuccess: (accepted) => {
      notifications.show({title: "Сборка поставлена в очередь", message: `Ревизия черновика ${accepted.revision}`, color: "green"});
      refresh();
    },
    onError: (error: Error & {status?: number}) => {
      notifications.show({
        title: error.status === 409 ? "Черновик изменился" : "Не удалось запустить сборку",
        message: error.status === 409 ? "Перезагрузите страницу и повторите." : error.message,
        color: "red",
      });
      if (error.status === 409) {
        void draft.refetch();
      }
    },
  });

  const cancel = useMutation({
    mutationFn: (buildId: string) =>
      unwrap(
        api.POST("/problems/{id}/builds/{buildId}/cancel", {
          params: {path: {id: problemId, buildId}, header: {"Idempotency-Key": crypto.randomUUID()}},
        }),
      ),
    onSuccess: refresh,
    onError: (error: Error) => notifications.show({title: "Не удалось отменить сборку", message: error.message, color: "red"}),
  });

  const hasActive = builds.data?.some((item) => ACTIVE_STATES.has(item.state)) ?? false;
  const packageItems = packages.data?.packages ?? [];

  return (
    <Box p="lg">
      <Stack gap="lg" maw={1000} mx="auto">
        <SectionPaper title="Сборка пакета">
          <Stack gap="sm">
            <Text size="sm" c="dimmed">
              Пакет собирается из снимка текущей ревизии черновика. Правки, сделанные после запуска, в этот пакет не
              попадут. Контесты и посылки привязываются к конкретному пакету.
            </Text>
            {problems.length > 0 && (
              <Alert color="yellow" variant="light" title="Перед сборкой исправьте">
                <List size="sm">
                  {problems.map((problem) => (
                    <List.Item key={problem}>{problem}</List.Item>
                  ))}
                </List>
              </Alert>
            )}
            <Group>
              <Button
                loading={build.isPending}
                disabled={!draft.data || problems.length > 0 || hasActive}
                onClick={() => build.mutate()}
              >
                Собрать пакет
              </Button>
              {draft.data && <Text size="sm" c="dimmed">Ревизия черновика: {draft.data.revision}</Text>}
            </Group>
          </Stack>
        </SectionPaper>

        <SectionPaper title="Сборки">
          {builds.isPending && (
            <Group justify="center" py="md">
              <Loader size="sm" />
            </Group>
          )}
          {builds.error && <Text c="red">{builds.error.message}</Text>}
          {builds.data && builds.data.length === 0 && <Text size="sm" c="dimmed">Сборок ещё не было.</Text>}
          {builds.data && builds.data.length > 0 && (
            <Table withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Ревизия</Table.Th>
                  <Table.Th>Состояние</Table.Th>
                  <Table.Th>Создана</Table.Th>
                  <Table.Th>Результат</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {builds.data.map((item) => (
                  <Table.Tr key={item.id}>
                    <Table.Td>{item.revision}</Table.Td>
                    <Table.Td>
                      <Badge color={STATE_BADGES[item.state].color} variant="light">
                        {STATE_BADGES[item.state].label}
                      </Badge>
                      {ACTIVE_STATES.has(item.state) && item.attempt > 0 && (
                        <Text size="xs" c="dimmed">
                          попытка {item.attempt} из {item.max_attempts}
                        </Text>
                      )}
                    </Table.Td>
                    <Table.Td>{formatDateTime(item.created_at)}</Table.Td>
                    <Table.Td>
                      <DiagnosticView diagnostic={item.diagnostic} />
                    </Table.Td>
                    <Table.Td>
                      {ACTIVE_STATES.has(item.state) && (
                        <Button size="xs" variant="subtle" color="red" loading={cancel.isPending && cancel.variables === item.id} onClick={() => cancel.mutate(item.id)}>
                          Отменить
                        </Button>
                      )}
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
        </SectionPaper>

        <SectionPaper title="Пакеты">
          {packages.error && <Text c="red">{packages.error.message}</Text>}
          {packages.isPending && (
            <Group justify="center" py="md">
              <Loader size="sm" />
            </Group>
          )}
          {!packages.isPending && !packages.error && packageItems.length === 0 && <Text size="sm" c="dimmed">Успешно собранных пакетов нет.</Text>}
          {packageItems.length > 0 && (
            <Table withTableBorder>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Ревизия</Table.Th>
                  <Table.Th>Хеш</Table.Th>
                  <Table.Th>Собран</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {packageItems.map((item) => (
                  <Table.Tr key={item.id}>
                    <Table.Td>{item.revision}</Table.Td>
                    <Table.Td>
                      <Text size="xs" ff="monospace" title={item.package_hash}>
                        {item.package_hash.slice(0, 8)}
                      </Text>
                    </Table.Td>
                    <Table.Td>{formatDateTime(item.created_at)}</Table.Td>
                    <Table.Td>
                      <Anchor size="sm" href={`/api/problems/${problemId}/packages/${item.id}/download`}>
                        Скачать
                      </Anchor>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
          {packages.data && packages.data.pagination.total_pages > 1 && (
            <Pagination
              value={packages.page}
              onChange={packages.setPage}
              total={packages.data.pagination.total_pages}
            />
          )}
        </SectionPaper>
      </Stack>
    </Box>
  );
};
