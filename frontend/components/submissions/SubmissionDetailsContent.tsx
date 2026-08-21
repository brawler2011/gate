"use client";

import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Card,
  Divider,
  Group,
  Paper,
  ScrollArea,
  SimpleGrid,
  Stack,
  Table,
  TableTbody,
  TableTd,
  TableTh,
  TableThead,
  TableTr,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconCheck,
  IconCopy,
  IconDownload,
  IconFileCode,
  IconInfoCircle,
  IconRefresh,
} from "@tabler/icons-react";
import Link from "next/link";
import React, {useState, type ReactNode} from "react";

import {CodeBlock} from "@/components/shared/CodeBlock";
import {
  LangNameToString,
  LangString,
  numberToLetters,
  ProblemTitle,
  ShortVerdictString,
  StateColor,
  StateString,
  TimeBeautify,
} from "@/lib/lib";

import type {SubmissionModel} from "@/contracts/core/v1";

interface SubmissionDetailsContentProps {
  submission: SubmissionModel;
  canRejudge?: boolean;
  onRejudge?: (submissionId: string) => Promise<void>;
  isRejudging?: boolean;
}

const copyToClipboard = (text: string, label: string) => {
  navigator.clipboard.writeText(text);
  notifications.show({
    title: "Скопировано",
    message: `${label} скопирован в буфер обмена`,
    color: "green",
    icon: <IconCheck size={16} />,
  });
};

const downloadTextFile = (content: string, filename: string) => {
  const blob = new Blob([content], {type: "text/plain;charset=utf-8"});
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

interface TestDataBlockProps {
  title: string;
  data?: string | null;
  filename?: string;
  badgeLabel?: string;
  badgeColor?: string;
}

const TestDataBlock = ({title, data, filename, badgeLabel, badgeColor}: TestDataBlockProps): ReactNode => {
  if (data === undefined || data === null) {
    return null;
  }

  const trimmed = data.trim();
  const displayData = trimmed.length === 0 ? "—" : data;

  return (
    <Paper withBorder p="xs" radius="sm" bg="var(--mantine-color-body)">
      <Group justify="space-between" mb="xs" wrap="nowrap">
        <Group gap="xs">
          <Text fw={600} size="sm">
            {title}
          </Text>
          {badgeLabel && (
            <Badge size="xs" color={badgeColor || "blue"} variant="light">
              {badgeLabel}
            </Badge>
          )}
        </Group>
        <Group gap={4}>
          <Tooltip label="Копировать в буфер">
            <ActionIcon
              size="sm"
              variant="subtle"
              color="gray"
              disabled={trimmed.length === 0}
              onClick={() => copyToClipboard(data, title)}
            >
              <IconCopy size={14} />
            </ActionIcon>
          </Tooltip>
          {filename && (
            <Tooltip label={`Скачать файл (${filename})`}>
              <ActionIcon
                size="sm"
                variant="subtle"
                color="blue"
                disabled={trimmed.length === 0}
                onClick={() => downloadTextFile(data, filename)}
              >
                <IconDownload size={14} />
              </ActionIcon>
            </Tooltip>
          )}
        </Group>
      </Group>
      <ScrollArea.Autosize mah={220} type="auto">
        <Box
          component="pre"
          m={0}
          p="xs"
          style={{
            fontFamily: "monospace",
            fontSize: "12px",
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
            backgroundColor: "var(--mantine-color-default-hover)",
            borderRadius: "var(--mantine-radius-xs)",
          }}
        >
          {displayData}
        </Box>
      </ScrollArea.Autosize>
    </Paper>
  );
};

export const SubmissionDetailsContent = ({
  submission,
  canRejudge,
  onRejudge,
  isRejudging,
}: SubmissionDetailsContentProps): ReactNode => {
  const [copiedCode, setCopiedCode] = useState(false);

  const handleCopyCode = () => {
    navigator.clipboard.writeText(submission.submission);
    setCopiedCode(true);
    setTimeout(() => setCopiedCode(false), 2000);
    notifications.show({
      title: "Успех",
      message: "Исходный код скопирован",
      color: "green",
      icon: <IconCheck size={16} />,
    });
  };

  const testDetails = submission.test_details;
  const failedDetails = testDetails?.failed_test_details;
  const errorLine = testDetails?.error_line;
  const compilerOutput = testDetails?.compiler_output;
  const tests = testDetails?.tests;

  const problemUrl =
    submission.organization_login && submission.contest_login
      ? `/${submission.organization_login}/${submission.contest_login}/${numberToLetters(submission.position)}`
      : `/problems/${submission.problem_id}`;

  return (
    <Stack gap="lg" w="100%">
      {/* Overview Card */}
      <Card withBorder radius="md" p="md" bg="var(--mantine-color-body)">
        <SimpleGrid cols={{base: 2, sm: 4}} spacing="md">
          <Box>
            <Text size="xs" c="dimmed">
              Отправитель
            </Text>
            <Link href={`/@${submission.username}`} style={{color: "inherit", textDecoration: "none"}}>
              <Text fw={600} c="blue" lineClamp={1}>
                {submission.username}
              </Text>
            </Link>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Задача
            </Text>
            <Link href={problemUrl} style={{color: "inherit", textDecoration: "none"}}>
              <Text fw={600} c="blue" lineClamp={1}>
                {ProblemTitle(submission.position, submission.problem_title)}
              </Text>
            </Link>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Язык
            </Text>
            <Badge variant="light" color="gray" tt="none">
              {LangString(submission.language)}
            </Badge>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Время отправки
            </Text>
            <Text size="sm" fw={500}>
              {TimeBeautify(submission.created_at)}
            </Text>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Вердикт
            </Text>
            <Group gap="xs" align="center">
              <Badge color={StateColor(submission.state)} variant="filled" size="md">
                {ShortVerdictString(submission.state, submission.failed_test)}
              </Badge>
              <Text size="xs" c={StateColor(submission.state)} fw={500}>
                {StateString(submission.state, submission.failed_test)}
              </Text>
            </Group>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Время выполнения
            </Text>
            <Text size="sm" fw={500}>
              {submission.time_stat} ms
            </Text>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Использованная память
            </Text>
            <Text size="sm" fw={500}>
              {submission.memory_stat} КБ
            </Text>
          </Box>
          <Box>
            <Text size="xs" c="dimmed">
              Баллы
            </Text>
            <Text size="sm" fw={600}>
              {submission.score}
            </Text>
          </Box>
        </SimpleGrid>

        {canRejudge && onRejudge && (
          <>
            <Divider my="sm" />
            <Group justify="flex-end">
              <Button
                variant="light"
                color="orange"
                size="xs"
                leftSection={<IconRefresh size={14} />}
                loading={isRejudging}
                onClick={() => onRejudge(submission.id)}
              >
                Перетестировать посылку
              </Button>
            </Group>
          </>
        )}
      </Card>

      {/* Compiler output / Compilation error */}
      {compilerOutput && (
        <Card withBorder radius="md" p="md" style={{borderColor: "var(--mantine-color-red-6)"}}>
          <Group justify="space-between" mb="xs">
            <Group gap="xs">
              <IconAlertCircle size={20} color="var(--mantine-color-red-6)" />
              <Title order={4} c="red.6">
                Вывод компилятора
              </Title>
              {errorLine && (
                <Badge color="red" variant="filled">
                  Ошибка на строке {errorLine}
                </Badge>
              )}
            </Group>
            <Button
              size="xs"
              variant="subtle"
              color="gray"
              leftSection={<IconCopy size={14} />}
              onClick={() => copyToClipboard(compilerOutput, "Лог компилятора")}
            >
              Копировать лог
            </Button>
          </Group>
          <Box
            component="pre"
            m={0}
            p="sm"
            style={{
              fontFamily: "monospace",
              fontSize: "12px",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
              backgroundColor: "var(--mantine-color-dark-8, #1a1b1e)",
              color: "#ff8787",
              borderRadius: "var(--mantine-radius-sm)",
            }}
          >
            {compilerOutput}
          </Box>
        </Card>
      )}

      {/* Failed Test Details Box */}
      {failedDetails && (
        <Card withBorder radius="md" p="md" bg="var(--mantine-color-body)">
          <Group justify="space-between" mb="sm">
            <Group gap="xs">
              <IconAlertTriangle size={20} color="var(--mantine-color-orange-6)" />
              <Title order={4}>
                Детали упавшего теста #{failedDetails.test_index}
              </Title>
            </Group>
            {errorLine && (
              <Badge color="red" variant="light">
                Строка {errorLine}
              </Badge>
            )}
          </Group>

          {failedDetails.is_truncated && (
            <Alert
              icon={<IconInfoCircle size={16} />}
              title="Данные теста обрезаны"
              color="orange"
              variant="light"
              mb="md"
            >
              Входные/выходные данные превысили лимит 128 КБ и были частично обрезаны для оптимизации отображения.
            </Alert>
          )}

          <Stack gap="sm">
            <TestDataBlock
              title="Входные данные"
              data={failedDetails.input}
              filename={`test_${failedDetails.test_index}.in`}
            />

            <TestDataBlock
              title="Вывод программы участника"
              data={failedDetails.output}
              filename={`test_${failedDetails.test_index}.out`}
            />

            {failedDetails.answer && (
              <TestDataBlock
                title="Правильный ответ"
                data={failedDetails.answer}
                filename={`test_${failedDetails.test_index}.ans`}
              />
            )}

            {failedDetails.checker_output && (
              <TestDataBlock
                title="Лог чекера"
                data={failedDetails.checker_output}
                badgeLabel="Сообщение валидатора"
                badgeColor="indigo"
              />
            )}

            {failedDetails.error_message && !failedDetails.checker_output && (
              <TestDataBlock
                title="Сообщение об ошибке"
                data={failedDetails.error_message}
                badgeColor="red"
              />
            )}
          </Stack>
        </Card>
      )}

      {/* Test Protocol Table */}
      {tests && tests.length > 0 && (
        <Card withBorder radius="md" p="md" bg="var(--mantine-color-body)">
          <Group justify="space-between" mb="sm">
            <Title order={4}>Протокол тестирования ({tests.length} тестов)</Title>
          </Group>
          <ScrollArea.Autosize mah={300} type="auto">
            <Table striped highlightOnHover withTableBorder verticalSpacing="xs">
              <TableThead>
                <TableTr>
                  <TableTh ta="center" style={{width: 60}}>#</TableTh>
                  <TableTh ta="center">Вердикт</TableTh>
                  <TableTh ta="center">Время</TableTh>
                  <TableTh ta="center">Память</TableTh>
                </TableTr>
              </TableThead>
              <TableTbody>
                {tests.map((t) => {
                  const isOk = t.verdict === "OK" || t.verdict === "AC" || t.verdict === "Accepted";
                  const isFailed = failedDetails?.test_index === t.test_index;
                  return (
                    <TableTr
                      key={t.test_index}
                      style={
                        isFailed
                          ? {backgroundColor: "rgba(255, 0, 0, 0.08)", fontWeight: 600}
                          : undefined
                      }
                    >
                      <TableTd ta="center">{t.test_index}</TableTd>
                      <TableTd ta="center">
                        <Badge
                          size="sm"
                          color={isOk ? "green" : "red"}
                          variant={isOk ? "light" : "filled"}
                        >
                          {t.verdict}
                        </Badge>
                      </TableTd>
                      <TableTd ta="center">{t.time_ms} ms</TableTd>
                      <TableTd ta="center">{t.memory_kb} КБ</TableTd>
                    </TableTr>
                  );
                })}
              </TableTbody>
            </Table>
          </ScrollArea.Autosize>
        </Card>
      )}

      {/* If test details are not available */}
      {!testDetails && (
        <Alert
          icon={<IconInfoCircle size={16} />}
          title="Детали тестов скрыты"
          color="gray"
          variant="light"
        >
          Подробный протокол тестирования недоступен для этой посылки или ограничен настройками контеста.
        </Alert>
      )}

      {/* Solution Code */}
      <Card withBorder radius="md" p="md" bg="var(--mantine-color-body)">
        <Group justify="space-between" align="center" mb="sm">
          <Group gap="xs">
            <IconFileCode size={20} />
            <Title order={4}>Исходный код решения</Title>
            {errorLine && (
              <Badge color="red" variant="filled">
                Подсветка строки {errorLine}
              </Badge>
            )}
          </Group>
          <Button
            size="xs"
            variant="light"
            color={copiedCode ? "green" : "blue"}
            leftSection={copiedCode ? <IconCheck size={14} /> : <IconCopy size={14} />}
            onClick={handleCopyCode}
          >
            {copiedCode ? "Скопировано!" : "Копировать код"}
          </Button>
        </Group>

        <CodeBlock
          code={submission.submission}
          language={LangNameToString(submission.language)}
          highlightLine={errorLine}
        />
      </Card>
    </Stack>
  );
};
