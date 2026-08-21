"use client";

import {Loader, Paper, Table, Text, Tooltip} from "@mantine/core";

import {
  ShortVerdictString,
  StateColor,
  StateString,
  TimeBeautify,
} from "@/lib/lib";
import {
  useSubmissionsWebSocket,
  type SubmissionWithProgress,
} from "@/lib/useSubmissionsWebSocket";
import {SubmissionDetailsModal} from "./SubmissionDetailsModal";

import styles from "./RecentSubmissionsTable.module.css";

import type {SubmissionsListItemModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";
import {useState} from "react";

const RECENT_SUBMISSIONS_LIMIT = 5;

type RecentSubmissionsTableProps = {
  submissions: SubmissionsListItemModel[];
  orgLogin?: string;
  contestLogin?: string;
  contestId: string;
  userId?: string;
  problemId?: string;
  wsUrl?: string;
  since?: number;
};

interface StatusCellProps {
  submission: SubmissionWithProgress;
  onOpenDetails?: () => void;
}

const StatusCell = ({submission, onOpenDetails}: StatusCellProps) => {
  const {state, progress, failed_test} = submission;

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
  const shortVerdict = ShortVerdictString(state, failed_test);
  const fullVerdict = StateString(state, failed_test);

  return (
    <Tooltip label={fullVerdict} withArrow>
      <Text
        c={StateColor(state)}
        fw={600}
        style={{cursor: onOpenDetails ? "pointer" : "default"}}
        onClick={onOpenDetails}
      >
        {shortVerdict}
      </Text>
    </Tooltip>
  );
};

export const RecentSubmissionsTable = ({
  submissions: initialSubmissions,
  orgLogin,
  contestLogin,
  contestId,
  userId,
  problemId,
  wsUrl,
  since,
}: RecentSubmissionsTableProps): ReactNode => {
  const [selectedSubmissionId, setSelectedSubmissionId] = useState<string | null>(null);

  // Enable WS only if wsUrl is provided and we have userId and problemId for filtering
  const enabled = Boolean(wsUrl && userId && problemId);

  const {submissions, highlightedIds} = useSubmissionsWebSocket({
    wsUrl,
    since,
    initialSubmissions,
    snapshotScope: "mine",
    filter: {
      orgLogin,
      contestLogin,
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
    <>
      <Paper
        shadow="sm"
        radius="md"
        p="md"
        withBorder
        bg="var(--mantine-color-gray-light)"
        style={{width: "100%"}}
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
                  <StatusCell
                    submission={submission as SubmissionWithProgress}
                    onOpenDetails={() => setSelectedSubmissionId(submission.id)}
                  />
                </Table.Td>
                <Table.Td ta="center">{submission.score}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>

      <SubmissionDetailsModal
        submissionId={selectedSubmissionId}
        opened={Boolean(selectedSubmissionId)}
        onClose={() => setSelectedSubmissionId(null)}
      />
    </>
  );
};
