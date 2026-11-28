import {ActionIcon, Alert, Button, Checkbox, FileInput, Group, Modal, NumberInput, Select, SegmentedControl, Stack, Text, TextInput, Textarea} from "@mantine/core";
import {IconTrash} from "@tabler/icons-react";
import {useState} from "react";

import {groupLabel, groupRows, isInteractive, toHundredths} from "../draft/draftOps";

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

// Null shows the test's own file; an empty text is a valid example.
type ExampleFieldProps = {label: string; value: string | null; onChange: (value: string | null) => void};

const ExampleField = ({label, value, onChange}: ExampleFieldProps): ReactNode => (
  <Stack gap={4}>
    <Checkbox label={label} checked={value !== null} onChange={(e) => onChange(e.currentTarget.checked ? "" : null)} />
    {value !== null && (
      <Textarea
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        autosize
        minRows={2}
        maxRows={10}
        styles={{input: {fontFamily: "var(--mantine-font-family-monospace)"}}}
      />
    )}
  </Stack>
);

// Arguments are passed as argv without shell expansion.
const parseArgs = (text: string): string[] => text.split(/\s+/).filter((part) => part !== "");

export const TestEditorModal = ({draft, test, isNew, saving, onClose, onSubmit}: Props): ReactNode => {
  const [groupId, setGroupId] = useState(test.group_id);
  const [sample, setSample] = useState(test.sample);
  const [pretest, setPretest] = useState(test.pretest);
  const [statementInput, setStatementInput] = useState(test.statement_input);
  const [statementOutput, setStatementOutput] = useState(test.statement_output);
  const [points, setPoints] = useState<number>(test.points);
  const [inputMode, setInputMode] = useState<"manual" | "generator">(test.input.mode);
  const [steps, setSteps] = useState(
    test.input.steps.length > 0
      ? test.input.steps.map((step) => ({generator: step.generator, args: step.args.join(" ")}))
      : [{generator: draft.generators[0]?.name ?? "", args: ""}],
  );
  const [generatedFile, setGeneratedFile] = useState(test.input.file);
  const setStep = (index: number, patch: Partial<{generator: string; args: string}>) =>
    setSteps((current) => current.map((step, i) => (i === index ? {...step, ...patch} : step)));
  const [answerMode, setAnswerMode] = useState(test.answer.mode);
  const [solution, setSolution] = useState(test.answer.solution ?? draft.main_solution);
  const [inputFile, setInputFile] = useState<File | null>(null);
  const [answerFile, setAnswerFile] = useState<File | null>(null);

  const interactive = isInteractive(draft.type);
  const programOptions = (programs: ProblemDraft["generators"]) => programs.map((p) => ({value: p.name, label: p.name}));

  const needsInputFile = inputMode === "manual" && !test.input.object_id && !inputFile;
  const needsAnswerFile = !interactive && answerMode === "provided" && !test.answer.object_id && !answerFile;
  const invalid =
    (inputMode === "generator" && steps.some((step) => !step.generator)) ||
    (!interactive && answerMode === "generated" && !solution) ||
    toHundredths(points) < 0;

  const buildAnswer = (): ProblemDraftTest["answer"] => {
    if (interactive) {
      return test.answer.mode === "provided" ? test.answer : {mode: "interactive", object_id: null, solution: null};
    }
    if (answerMode === "provided") {
      return {mode: "provided", object_id: test.answer.object_id, solution: null};
    }
    return {mode: "generated", object_id: null, solution};
  };

  const submit = () => {
    const next: ProblemDraftTest = {
      ...test,
      group_id: groupId,
      sample,
      pretest,
      statement_input: sample ? statementInput : null,
      statement_output: sample ? statementOutput : null,
      points: toHundredths(points) / 100,
      input:
        inputMode === "manual"
          ? {mode: "manual", object_id: test.input.object_id, steps: [], file: ""}
          : {mode: "generator", object_id: null, steps: steps.map((step) => ({generator: step.generator, args: parseArgs(step.args)})), file: generatedFile.trim()},
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
            label="Группа"
            data={groupRows(draft).map(({group, depth}) => ({value: group.id, label: "\u2003".repeat(depth) + groupLabel(group)}))}
            value={groupId}
            allowDeselect={false}
            onChange={(value) => value && setGroupId(value)}
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
        {sample && (
          <Stack gap="xs">
            {interactive && statementInput === null && (
              <Alert color="yellow">
                Пример не показывается в условии: файлы интерактивного теста — данные интерактора.
                Задайте ввод для условия.
              </Alert>
            )}
            <Group grow align="flex-start">
              <ExampleField label="Свой ввод в условии" value={statementInput} onChange={setStatementInput} />
              <ExampleField label="Свой вывод в условии" value={statementOutput} onChange={setStatementOutput} />
            </Group>
          </Stack>
        )}
        <Checkbox
          label="Претест (на контестах с претестами до конца проверяется только на претестах)"
          checked={pretest}
          onChange={(e) => setPretest(e.currentTarget.checked)}
        />

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
              {steps.map((step, index) => (
                <Group key={index} align="flex-end" wrap="nowrap">
                  <Select
                    label={index === 0 ? "Генератор" : `Шаг ${index + 1}: читает вывод предыдущего`}
                    data={programOptions(draft.generators)}
                    value={step.generator || null}
                    onChange={(value) => setStep(index, {generator: value ?? ""})}
                    placeholder="Не выбран"
                    w={200}
                  />
                  <TextInput
                    label="Аргументы"
                    description={index === 0 ? "Передаются как argv, разделитель — пробел" : undefined}
                    value={step.args}
                    onChange={(e) => setStep(index, {args: e.currentTarget.value})}
                    style={{flex: 1}}
                  />
                  {steps.length > 1 && (
                    <ActionIcon variant="subtle" color="red" aria-label="Убрать шаг" onClick={() => setSteps((current) => current.filter((_, i) => i !== index))}>
                      <IconTrash size={16} />
                    </ActionIcon>
                  )}
                </Group>
              ))}
              <Group>
                <Button size="xs" variant="light" onClick={() => setSteps((current) => [...current, {generator: "", args: ""}])}>
                  Добавить шаг (|)
                </Button>
              </Group>
              <TextInput
                label="Файл"
                description="Пусто — вход это вывод последнего шага. Иначе — файл, который запуск пишет в рабочий каталог; тесты с одинаковыми шагами строятся одним запуском."
                value={generatedFile}
                onChange={(e) => setGeneratedFile(e.currentTarget.value)}
              />
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
                <Select label="Решение" data={programOptions(draft.solutions)} value={solution} onChange={setSolution} placeholder="Не выбрано" />
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
