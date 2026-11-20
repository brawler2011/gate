"use client";

import {
  ActionIcon,
  Anchor,
  Badge,
  Button,
  FileInput,
  Group,
  NumberInput,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {IconTrash} from "@tabler/icons-react";
import {useState} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {useProblemBuilds} from "@/components/workshop/WorkshopPackagesTab";

import {
  removeCheckerTest,
  removeValidatorTest,
  updateCheckerTest,
  updateValidatorTest,
} from "./draft/draftOps";
import {
  objectUrl,
  useSaveDraft,
  useUploadToDraft,
  type UploadSlot,
} from "./draft/useDraft";

import type {
  ProblemBuildProgramTest,
  ProblemDraft,
  ProblemDraftCheckerTest,
} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type CheckerVerdict = ProblemDraftCheckerTest["verdict"];

const CHECKER_VERDICTS: { value: CheckerVerdict; label: string }[] = [
  {value: "OK", label: "OK — принято"},
  {value: "WA", label: "WA — неправильный ответ"},
  {value: "PE", label: "PE — ошибка представления"},
  {value: "FAIL", label: "FAIL — сбой чекера"},
  {value: "POINTS", label: "POINTS — частичные баллы"},
];

const VALIDITY = [
  {value: "valid", label: "Корректный"},
  {value: "invalid", label: "Некорректный"},
];

const SLOT_LABELS: Record<UploadSlot, string> = {
  asset: "Файл",
  input: "Вход",
  output: "Вывод участника",
  answer: "Ответ",
};

const NIL = "00000000-0000-0000-0000-000000000000";

// Results of the newest build that ran the checker and validator tests.
const useProgramTestResults = (
  problemId: string,
): {
  revision: number;
  results: Map<string, ProblemBuildProgramTest>;
} | null => {
  const build = useProblemBuilds(problemId, true).data?.find(
    (item) => item.program_tests,
  );
  return build?.program_tests
    ? {
      revision: build.revision,
      results: new Map(build.program_tests.map((r) => [r.id, r])),
    }
    : null;
};

const ResultCell = ({
  result,
}: {
  result: ProblemBuildProgramTest | undefined;
}): ReactNode =>
  result ? (
    <Tooltip
      label={
        result.mismatch
          ? `Не совпадает с ожидаемым: ${result.mismatch}`
          : "Совпадает с ожидаемым"
      }
    >
      <Badge
        size="sm"
        variant="light"
        color={result.mismatch ? "red" : "green"}
      >
        {result.verdict}
        {result.points !== null && ` ${result.points}`}
      </Badge>
    </Tooltip>
  ) : (
    <Text size="sm" c="dimmed">
      —
    </Text>
  );

const FileLinks = ({
  problemId,
  files,
}: {
  problemId: string;
  files: [string, string][];
}): ReactNode => (
  <Group gap="xs" wrap="nowrap">
    {files.map(([label, id]) => (
      <Anchor
        key={label}
        size="sm"
        href={objectUrl(problemId, id)}
        target="_blank"
        rel="noreferrer"
      >
        {label}
      </Anchor>
    ))}
  </Group>
);

type NewTestProps = {
  slots: UploadSlot[];
  uploading: boolean;
  expectation: ReactNode;
  onAdd: (files: Partial<Record<UploadSlot, File>>) => Promise<boolean>;
};

const NewTestForm = ({
  slots,
  uploading,
  expectation,
  onAdd,
}: NewTestProps): ReactNode => {
  const [files, setFiles] = useState<Partial<Record<UploadSlot, File>>>({});
  // A new FileInput forgets the files of the test just added.
  const [generation, setGeneration] = useState(0);
  return (
    <Group align="flex-end" wrap="wrap" key={generation}>
      {slots.map((slot) => (
        <FileInput
          key={slot}
          label={SLOT_LABELS[slot]}
          placeholder="Выберите файл"
          onChange={(file) =>
            setFiles((current) => ({...current, [slot]: file ?? undefined}))
          }
          w={180}
        />
      ))}
      {expectation}
      <Button
        variant="light"
        loading={uploading}
        disabled={slots.some((slot) => !files[slot])}
        onClick={() =>
          void onAdd(files).then((ok) => {
            if (ok) {
              setFiles({});
              setGeneration((g) => g + 1);
            }
          })
        }
      >
        Добавить тест
      </Button>
    </Group>
  );
};

type Props = { draft: ProblemDraft; problemId: string };

const HINT =
  "Тесты запускаются при каждой сборке; если результат не совпадает с ожидаемым, сборка завершается ошибкой.";

export const CheckerTests = ({draft, problemId}: Props): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const {upload, isUploading} = useUploadToDraft(problemId);
  const last = useProgramTestResults(problemId);
  const [verdict, setVerdict] = useState<CheckerVerdict>("OK");
  const tests = draft.checker?.tests ?? [];
  return (
    <SectionPaper title="Тесты чекера">
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          {HINT}
          {last && ` Результаты — сборка ревизии ${last.revision}.`}
        </Text>
        {tests.length > 0 && (
          <Table withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={50}>№</Table.Th>
                <Table.Th>Файлы</Table.Th>
                <Table.Th>Ожидаемый вердикт</Table.Th>
                <Table.Th>Ожидаемые баллы</Table.Th>
                <Table.Th>Результат</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {tests.map((test, index) => (
                <Table.Tr key={test.id}>
                  <Table.Td>{index + 1}</Table.Td>
                  <Table.Td>
                    <FileLinks
                      problemId={problemId}
                      files={[
                        ["вход", test.input],
                        ["вывод", test.output],
                        ["ответ", test.answer],
                      ]}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Select
                      size="xs"
                      data={CHECKER_VERDICTS}
                      value={test.verdict}
                      allowDeselect={false}
                      onChange={(value) =>
                        value &&
                        void save((current) =>
                          updateCheckerTest(current, test.id, {
                            verdict: value as CheckerVerdict,
                            points: value === "POINTS" ? test.points : null,
                          }),
                        )
                      }
                      w={220}
                    />
                  </Table.Td>
                  <Table.Td>
                    {test.verdict === "POINTS" && (
                      <NumberInput
                        size="xs"
                        w={100}
                        placeholder="любые"
                        defaultValue={test.points ?? ""}
                        min={0}
                        decimalScale={2}
                        hideControls
                        onBlur={(e) => {
                          const text = e.currentTarget.value
                            .replace(",", ".")
                            .trim();
                          const points =
                            text === ""
                              ? null
                              : Math.round(Number(text) * 100) / 100;
                          if (
                            points !== test.points &&
                            (points === null || Number.isFinite(points))
                          ) {
                            void save((current) =>
                              updateCheckerTest(current, test.id, {points}),
                            );
                          }
                        }}
                      />
                    )}
                  </Table.Td>
                  <Table.Td>
                    <ResultCell result={last?.results.get(test.id)} />
                  </Table.Td>
                  <Table.Td>
                    <ActionIcon
                      variant="subtle"
                      color="red"
                      aria-label="Удалить тест"
                      disabled={isSaving}
                      onClick={() =>
                        void save((current) =>
                          removeCheckerTest(current, test.id),
                        )
                      }
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
        <NewTestForm
          slots={["input", "output", "answer"]}
          uploading={isUploading}
          expectation={
            <Select
              label="Ожидаемый вердикт"
              data={CHECKER_VERDICTS}
              value={verdict}
              allowDeselect={false}
              onChange={(value) => value && setVerdict(value as CheckerVerdict)}
              w={220}
            />
          }
          onAdd={(files) =>
            upload({
              change: {
                checker_test: {
                  id: crypto.randomUUID(),
                  input: NIL,
                  output: NIL,
                  answer: NIL,
                  verdict,
                  points: null,
                },
              },
              files,
            }).then(
              () => true,
              () => false,
            )
          }
        />
      </Stack>
    </SectionPaper>
  );
};

export const ValidatorTests = ({
  draft,
  problemId,
  name,
}: Props & { name: string }): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const {upload, isUploading} = useUploadToDraft(problemId);
  const last = useProgramTestResults(problemId);
  const [valid, setValid] = useState(false);
  const [group, setGroup] = useState("");
  const tests = draft.validators.find((v) => v.name === name)?.tests ?? [];
  return (
    <SectionPaper title="Тесты валидатора">
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          {HINT} Группа передаётся валидатору как --group.
          {last && ` Результаты — сборка ревизии ${last.revision}.`}
        </Text>
        {tests.length > 0 && (
          <Table withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={50}>№</Table.Th>
                <Table.Th>Вход</Table.Th>
                <Table.Th>Ожидается</Table.Th>
                <Table.Th>Группа</Table.Th>
                <Table.Th>Результат</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {tests.map((test, index) => (
                <Table.Tr key={test.id}>
                  <Table.Td>{index + 1}</Table.Td>
                  <Table.Td>
                    <FileLinks
                      problemId={problemId}
                      files={[["файл", test.input]]}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Select
                      size="xs"
                      data={VALIDITY}
                      value={test.valid ? "valid" : "invalid"}
                      allowDeselect={false}
                      onChange={(value) =>
                        value &&
                        void save((current) =>
                          updateValidatorTest(current, name, test.id, {
                            valid: value === "valid",
                          }),
                        )
                      }
                      w={160}
                    />
                  </Table.Td>
                  <Table.Td>
                    <TextInput
                      size="xs"
                      w={140}
                      placeholder="без группы"
                      defaultValue={test.group}
                      onBlur={(e) => {
                        const value = e.currentTarget.value.trim();
                        if (value !== test.group) {
                          void save((current) =>
                            updateValidatorTest(current, name, test.id, {
                              group: value,
                            }),
                          );
                        }
                      }}
                    />
                  </Table.Td>
                  <Table.Td>
                    <ResultCell result={last?.results.get(test.id)} />
                  </Table.Td>
                  <Table.Td>
                    <ActionIcon
                      variant="subtle"
                      color="red"
                      aria-label="Удалить тест"
                      disabled={isSaving}
                      onClick={() =>
                        void save((current) =>
                          removeValidatorTest(current, name, test.id),
                        )
                      }
                    >
                      <IconTrash size={16} />
                    </ActionIcon>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
        <NewTestForm
          slots={["input"]}
          uploading={isUploading}
          expectation={
            <>
              <Select
                label="Ожидается"
                data={VALIDITY}
                value={valid ? "valid" : "invalid"}
                allowDeselect={false}
                onChange={(value) => setValid(value === "valid")}
                w={160}
              />
              <TextInput
                label="Группа"
                placeholder="без группы"
                value={group}
                onChange={(e) => setGroup(e.currentTarget.value)}
                w={140}
              />
            </>
          }
          onAdd={(files) =>
            upload({
              change: {
                validator: name,
                validator_test: {
                  id: crypto.randomUUID(),
                  input: NIL,
                  valid,
                  group: group.trim(),
                },
              },
              files,
            }).then(
              () => true,
              () => false,
            )
          }
        />
      </Stack>
    </SectionPaper>
  );
};
