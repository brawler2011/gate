"use client";

import { Anchor, Loader, Table, Text } from "@mantine/core";
import Link from "next/link";
import type { SubmissionsListItemModel } from "@contracts/core/v1";
import { StateColor, StateString, TimeBeautify } from "@/lib/lib";
import { useSubmissionsWebSocket, type SubmissionWithProgress } from "@/lib/useSubmissionsWebSocket";
import { notifications } from "@mantine/notifications";
import { useEffect, useRef } from "react";
import styles from "./styles.module.css";

const RECENT_SUBMISSIONS_LIMIT = 5;

type RecentSubmissionsTableProps = {
  submissions: SubmissionsListItemModel[];
  contestId: string;
  userId?: string;
  problemId?: string;
  wsUrl?: string;
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
    return (
      <div className={styles.statusCellVertical}>
        <div className={styles.statusCell}>
          <Loader size="xs" />
          <span>{progress.testNumber}/{progress.totalTests}</span>
        </div>
        <div className={styles.miniProgressBar}>
          <div
            className={progress.hasFailed ? styles.miniProgressFillError : styles.miniProgressFill}
            style={{ width: `${progress.totalTests > 0 ? (progress.testNumber / progress.totalTests) * 100 : 0}%` }}
          />
        </div>
      </div>
    );
  }

  // Final verdict
  // DEBUG: Log failed_test value
  console.log('Submission verdict:', { id: submission.id, state, failed_test: submission.failed_test });
  
  return (
    <Text c={StateColor(state)} fw={500}>
      {StateString(state, submission.failed_test)}
    </Text>
  );
};

export function RecentSubmissionsTable({
  submissions: initialSubmissions,
  contestId,
  userId,
  problemId,
  wsUrl,
}: RecentSubmissionsTableProps) {
  const prevErrorRef = useRef(false);
  
  // Enable WS only if wsUrl is provided and we have userId and problemId for filtering
  const enabled = Boolean(wsUrl && userId && problemId);

  const { submissions, connectionError, highlightedIds } = useSubmissionsWebSocket({
    wsUrl: wsUrl || '',
    initialSubmissions,
    filter: {
      contestId,
      userId,
      problemId,
    },
    pageSize: RECENT_SUBMISSIONS_LIMIT,
    enabled,
  });

  // Show toast notification when connection error occurs
  useEffect(() => {
    if (connectionError && !prevErrorRef.current) {
      notifications.show({
        title: 'Соединение потеряно',
        message: 'Не удалось подключиться к серверу обновлений.',
        color: 'orange',
        autoClose: 5000,
      });
    }
    prevErrorRef.current = connectionError;
  }, [connectionError]);

  // Use WS submissions if enabled, otherwise use initial submissions
  const displaySubmissions = enabled ? submissions.slice(0, RECENT_SUBMISSIONS_LIMIT) : initialSubmissions.slice(0, RECENT_SUBMISSIONS_LIMIT);

  if (displaySubmissions.length === 0) {
    return null;
  }

  return (
    <>
      <Text fw={500}>
        Последние посылки{" "}
        <Anchor
          component={Link}
          href={`/contests/${contestId}/mysubmissions?order=desc&userId=${userId}`}
          fs="italic"
          c="var(--mantine-color-text)"
          fw={500}
        >
          (посмотреть все)
        </Anchor>
        :
      </Text>
      <Table verticalSpacing="xs" horizontalSpacing="sm">
        <Table.Thead>
          <Table.Tr>
            <Table.Th ta="center">Дата отправки</Table.Th>
            <Table.Th ta="center" className={styles.statusColumn}>Статус</Table.Th>
            <Table.Th ta="center">Баллы</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {displaySubmissions.map((submission) => (
            <Table.Tr 
              key={submission.id}
              className={highlightedIds.has(submission.id) ? styles.rowHighlight : undefined}
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
    </>
  );
}
