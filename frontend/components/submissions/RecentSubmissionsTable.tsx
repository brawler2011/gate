"use client";

import { StateColor, StateString, TimeBeautify } from "@/lib/lib";
import {
  useSubmissionsWebSocket,
  type SubmissionWithProgress,
} from "@/lib/useSubmissionsWebSocket";
import type { SubmissionsListItemModel } from "@contracts/core/v1";
import { Loader, Paper, Table, Text } from "@mantine/core";
import styles from "./RecentSubmissionsTable.module.css";

const RECENT_SUBMISSIONS_LIMIT = 5;

type RecentSubmissionsTableProps = {
  submissions: SubmissionsListItemModel[];
  contestId: string;
  userId?: string;
  problemId?: string;
  wsUrl?: string;
  since?: number;
};

interface StatusCellProps {
  submission: SubmissionWithProgress;
}

const StatusCell = ({ submission }: StatusCellProps) => {
  const { state, progress } = submission;

  // State 1 = Saved (in queue, not yet testing)
  if (state === 1 && !progress) {
    return (
      <div className={styles.queueStatus}>
        <Loader size="xs" />
        <span>В очереди</span>
      </div>
    );
  }

  // Currently testing (has progress)
  if (progress) {
    const phaseLabels = {
      queued: "В очереди",
      compiling: "Компиляция",
      testing: `Тест ${progress.testNumber}`,
    };

    return (
      <div className={styles.statusCell}>
        <Loader size="xs" />
        <span>{phaseLabels[progress.phase]}</span>
      </div>
    );
  }

  // Final verdict
  return (
    <Text c={StateColor(state)} fw={500}>
      {StateString(state)}
    </Text>
  );
};

export function RecentSubmissionsTable({
  submissions: initialSubmissions,
  contestId,
  userId,
  problemId,
  wsUrl,
  since,
}: RecentSubmissionsTableProps) {
  // Enable WS only if wsUrl is provided and we have userId and problemId for filtering
  const enabled = Boolean(wsUrl && userId && problemId);

  const { submissions, highlightedIds } = useSubmissionsWebSocket({
    wsUrl,
    since,
    initialSubmissions,
    snapshotScope: "mine",
    filter: {
      contestId,
      userId,
      problemId,
    },
    pageSize: RECENT_SUBMISSIONS_LIMIT,
    enabled,
  });

  // Use WS submissions if enabled, otherwise use initial submissions
  const displaySubmissions = enabled
    ? submissions.slice(0, RECENT_SUBMISSIONS_LIMIT)
    : initialSubmissions.slice(0, RECENT_SUBMISSIONS_LIMIT);

  if (displaySubmissions.length === 0) {
    return null;
  }

  return (
    <Paper
      shadow="sm"
      radius="md"
      p="md"
      withBorder
      bg="var(--mantine-color-gray-light)"
      style={{ width: "100%" }}
    >
      <Table verticalSpacing="xs" horizontalSpacing="sm">
        <Table.Thead>
          <Table.Tr>
            <Table.Th ta="center">Дата отправки</Table.Th>
            <Table.Th ta="center" className={styles.statusColumn}>
              Статус
            </Table.Th>
            <Table.Th ta="center">Баллы</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {displaySubmissions.map((submission) => (
            <Table.Tr
              key={submission.id}
              className={
                highlightedIds.has(submission.id)
                  ? styles.rowHighlight
                  : undefined
              }
            >
              <Table.Td ta="center">
                <Text fw={500}>{TimeBeautify(submission.created_at)}</Text>
              </Table.Td>
              <Table.Td ta="center" className={styles.statusColumn}>
                <StatusCell submission={submission as SubmissionWithProgress} />
              </Table.Td>
              <Table.Td ta="center">{submission.score}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Paper>
  );
}
