"use client";

import {Box, Button, Divider, Grid, Group, NumberInput, Select, Stack, Switch, Text, TextInput} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useQuery, useQueryClient} from "@tanstack/react-query";
import {useState, useTransition} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {api, unwrap} from "@/lib/api";

import {DraftGate} from "./draft/DraftGate";
import {formatPoints} from "./draft/draftOps";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const PROBLEM_TYPE_OPTIONS = [
  {value: "pass-fail", label: "Pass-Fail"},
  {value: "scoring", label: "Scoring"},
  {value: "interactive", label: "Interactive"},
];

type Props = {
  problemId: string;
};

const DraftForm = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [title, setTitle] = useState(draft.title);
  const [shortName, setShortName] = useState(draft.short_name);
  const [type, setType] = useState<ProblemDraft["type"]>(draft.type);
  const [timeLimit, setTimeLimit] = useState<number>(draft.time_limit_ms);
  const [memoryLimit, setMemoryLimit] = useState<number>(draft.memory_limit_mb);
  const [mainSolution, setMainSolution] = useState(draft.main_solution);

  const changed =
    title !== draft.title ||
    shortName !== draft.short_name ||
    type !== draft.type ||
    timeLimit !== draft.time_limit_ms ||
    memoryLimit !== draft.memory_limit_mb ||
    mainSolution !== draft.main_solution;

  const handleSave = async () => {
    const ok = await save((current) => ({
      ...current,
      title: title.trim(),
      short_name: shortName.trim(),
      type,
      time_limit_ms: timeLimit,
      memory_limit_mb: memoryLimit,
      main_solution: mainSolution,
    }));
    if (ok) {
      notifications.show({title: "Сохранено", message: "Черновик обновлён", color: "green"});
    }
  };

  return (
    <SectionPaper title="Черновик задачи">
      <Stack gap="md">
        <Grid gutter="md">
          <Grid.Col span={{base: 12, sm: 8}}>
            <TextInput label="Название" value={title} onChange={(e) => setTitle(e.currentTarget.value)} required />
          </Grid.Col>
          <Grid.Col span={{base: 12, sm: 4}}>
            <TextInput label="Короткое имя" value={shortName} onChange={(e) => setShortName(e.currentTarget.value)} required />
          </Grid.Col>
          <Grid.Col span={{base: 12, sm: 6}}>
            <Select
              label="Тип задачи"
              data={PROBLEM_TYPE_OPTIONS}
              value={type}
              allowDeselect={false}
              onChange={(value) => setType((value as ProblemDraft["type"] | null) ?? "pass-fail")}
            />
          </Grid.Col>
          {type === "scoring" && (
            <Grid.Col span={{base: 12, sm: 6}}>
              <TextInput
                label="Максимальный балл"
                description="Сумма весов всех тестов"
                value={draft.max_score === null ? "—" : formatPoints(draft.max_score)}
                readOnly
              />
            </Grid.Col>
          )}
          <Grid.Col span={{base: 12, sm: 6}}>
            <NumberInput
              label="Ограничение по времени (мс)"
              value={timeLimit}
              onChange={(value) => setTimeLimit(typeof value === "number" ? value : timeLimit)}
              min={100}
              max={60000}
              step={100}
            />
          </Grid.Col>
          <Grid.Col span={{base: 12, sm: 6}}>
            <NumberInput
              label="Ограничение по памяти (МБ)"
              value={memoryLimit}
              onChange={(value) => setMemoryLimit(typeof value === "number" ? value : memoryLimit)}
              min={16}
              max={2048}
              step={16}
            />
          </Grid.Col>
        </Grid>

        <Divider label="Компоненты сборки" labelPosition="left" />
        <Grid gutter="md">
          <Grid.Col span={{base: 12, sm: 6}}>
            <Select
              label="Главное решение"
              placeholder="Не выбрано"
              clearable
              data={draft.solutions.map((s) => ({value: s.name, label: s.name}))}
              value={mainSolution}
              onChange={setMainSolution}
            />
          </Grid.Col>
        </Grid>
        <Text size="xs" c="dimmed">
          Главное решение формирует ответы для тестов в режиме «генерируется». Чекер и интерактор у
          задачи единственные и задаются на своих вкладках.
        </Text>

        <Group justify="flex-end">
          <Button disabled={!changed} loading={isSaving} onClick={() => void handleSave()}>
            Сохранить
          </Button>
        </Group>
      </Stack>
    </SectionPaper>
  );
};

const CatalogSettings = ({problemId}: Props): ReactNode => {
  const queryClient = useQueryClient();
  const [isPending, startTransition] = useTransition();
  const {data} = useQuery({
    queryKey: ["problem", problemId],
    queryFn: () => unwrap(api.GET("/problems/{id}", {params: {path: {id: problemId}}})),
  });
  const isTemplate = data?.problem.is_template ?? false;

  const toggle = (checked: boolean) => {
    startTransition(async () => {
      const {error} = await api.PATCH("/problems/{id}", {
        params: {path: {id: problemId}},
        body: {is_template: checked},
      });
      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message ?? "Не удалось обновить настройки шаблона",
          color: "red",
        });
        return;
      }
      await queryClient.invalidateQueries({queryKey: ["problem", problemId]});
    });
  };

  return (
    <SectionPaper title="Каталог">
      <Switch
        label="Шаблонная задача (доступна при создании других задач)"
        description="Для шаблона требуется хотя бы один успешно собранный пакет"
        checked={isTemplate}
        disabled={!data || isPending}
        onChange={(event) => toggle(event.currentTarget.checked)}
      />
    </SectionPaper>
  );
};

export const WorkshopGeneralTab = ({problemId}: Props): ReactNode => (
  <Box p="lg">
    <Stack gap="lg" maw={900} mx="auto">
      <DraftGate problemId={problemId}>
        {(draft) => <DraftForm key={draft.revision} draft={draft} problemId={problemId} />}
      </DraftGate>
      <CatalogSettings problemId={problemId} />
    </Stack>
  </Box>
);
