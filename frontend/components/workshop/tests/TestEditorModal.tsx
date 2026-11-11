import {Button, Checkbox, FileInput, Group, Modal, NumberInput, Select, SegmentedControl, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";

import {toHundredths} from "../draft/draftOps";

import type {ProblemDraft, ProblemDraftTest} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

export type TestFiles = {input: File | null; answer: File | null};

type Props = {
  draft: ProblemDraft;
  test: ProblemDraftTest;
  isNew: boolean;
  saving: boolean;
  onClose: () => void;
  onSubmit: (test: ProblemDraftTest, files: TestFiles) => void;
};

/** Splits generator arguments on whitespace; the generator receives them as argv without a shell. */
const parseArgs = (text: string): string[] => text.split(/\s+/).filter((part) => part !== "");

export const TestEditorModal = ({draft, test, isNew, saving, onClose, onSubmit}: Props): ReactNode => {
  const [subtaskId, setSubtaskId] = useState(test.subtask_id);
  const [sample, setSample] = useState(test.sample);
  const [points, setPoints] = useState<number>(test.points);
  const [inputMode, setInputMode] = useState<"manual" | "generator">(test.input.mode);
  const [generatorId, setGeneratorId] = useState(test.input.component_id);
  const [args, setArgs] = useState(test.input.args.join(" "));
  const [seed, setSeed] = useState<number>(test.input.seed ?? 1);
  const [answerMode, setAnswerMode] = useState(test.answer.mode);
  const [solutionId, setSolutionId] = useState(test.answer.component_id ?? draft.main_solution_id);
  const [inputFile, setInputFile] = useState<File | null>(null);
  const [answerFile, setAnswerFile] = useState<File | null>(null);

  const interactive = draft.type === "interactive";
  const componentOptions = (role: "generator" | "solution") =>
    draft.components.filter((c) => c.role === role).map((c) => ({value: c.id, label: c.path}));

  const needsInputFile = inputMode === "manual" && !test.input.object_id && !inputFile;
  const needsAnswerFile = !interactive && answerMode === "provided" && !test.answer.object_id && !answerFile;
  const invalid =
    (inputMode === "generator" && !generatorId) ||
    (!interactive && answerMode === "generated" && !solutionId) ||
    toHundredths(points) < 0;

  const buildAnswer = (): ProblemDraftTest["answer"] => {
    if (interactive) {
      return {mode: "interactive", object_id: null, component_id: null};
    }
    if (answerMode === "provided") {
      return {mode: "provided", object_id: test.answer.object_id, component_id: null};
    }
    return {mode: "generated", object_id: null, component_id: solutionId};
  };

  const submit = () => {
    const next: ProblemDraftTest = {
      ...test,
      subtask_id: subtaskId,
      sample,
      points: toHundredths(points) / 100,
      input:
        inputMode === "manual"
          ? {mode: "manual", object_id: test.input.object_id, component_id: null, args: [], seed: null}
          : {mode: "generator", object_id: null, component_id: generatorId, args: parseArgs(args), seed},
      answer: buildAnswer(),
    };
    onSubmit(next, {
      input: inputMode === "manual" ? inputFile : null,
      answer: !interactive && answerMode === "provided" ? answerFile : null,
    });
  };

  return (
    <Modal opened onClose={onClose} title={isNew ? "Новый тест" : `Тест ${test.ordinal}`} size="lg" centered>
      <Stack gap="md">
        <Group grow align="flex-start">
          <Select
            label="Подзадача"
            data={draft.subtasks.map((s) => ({value: s.id, label: s.name}))}
            value={subtaskId}
            allowDeselect={false}
            onChange={(value) => value && setSubtaskId(value)}
          />
          <NumberInput
            label="Вес (баллы)"
            description="С точностью до 0.01"
            value={points}
            onChange={(value) => setPoints(typeof value === "number" ? value : 0)}
            min={0}
            decimalScale={2}
            step={1}
          />
        </Group>
        <Checkbox label="Пример (показывается в условии)" checked={sample} onChange={(e) => setSample(e.currentTarget.checked)} />

        <Stack gap="xs">
          <Text fw={600} size="sm">
            Входные данные
          </Text>
          <SegmentedControl
            data={[
              {value: "manual", label: "Файл"},
              {value: "generator", label: "Генератор"},
            ]}
            value={inputMode}
            onChange={(value) => setInputMode(value as "manual" | "generator")}
          />
          {inputMode === "manual" ? (
            <FileInput
              label={test.input.object_id ? "Заменить файл входа" : "Файл входа"}
              value={inputFile}
              onChange={setInputFile}
              clearable
            />
          ) : (
            <>
              <Select label="Генератор" data={componentOptions("generator")} value={generatorId} onChange={setGeneratorId} placeholder="Не выбран" />
              <TextInput label="Аргументы" description="Передаются генератору как argv, разделитель — пробел" value={args} onChange={(e) => setArgs(e.currentTarget.value)} />
              <NumberInput label="Seed" value={seed} onChange={(value) => setSeed(typeof value === "number" ? value : seed)} allowDecimal={false} />
            </>
          )}
        </Stack>

        <Stack gap="xs">
          <Text fw={600} size="sm">
            Ответ
          </Text>
          {interactive ? (
            <Text size="sm" c="dimmed">
              Главное решение взаимодействует с интерактором, файл ответа не нужен.
            </Text>
          ) : (
            <>
              <SegmentedControl
                data={[
                  {value: "generated", label: "Решение"},
                  {value: "provided", label: "Файл"},
                ]}
                value={answerMode}
                onChange={(value) => setAnswerMode(value as "provided" | "generated")}
              />
              {answerMode === "provided" ? (
                <FileInput
                  label={test.answer.object_id ? "Заменить файл ответа" : "Файл ответа"}
                  value={answerFile}
                  onChange={setAnswerFile}
                  clearable
                />
              ) : (
                <Select label="Решение" data={componentOptions("solution")} value={solutionId} onChange={setSolutionId} placeholder="Не выбрано" />
              )}
            </>
          )}
        </Stack>

        {(needsInputFile || needsAnswerFile) && (
          <Text size="xs" c="dimmed">
            Тест можно сохранить без файлов, но собрать пакет получится только после их загрузки.
          </Text>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Отмена
          </Button>
          <Button disabled={invalid} loading={saving} onClick={submit}>
            Сохранить
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
