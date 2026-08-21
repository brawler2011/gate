"use client";

import {
  ActionIcon,
  Group,
  Loader,
  Table,
  TableTbody,
  TableTd,
  TableTh,
  TableThead,
  TableTr,
  Text,
  Tooltip,
  Transition,
  TableScrollContainer,
} from "@mantine/core";
import {IconRefresh} from "@tabler/icons-react";
import Link from "next/link";
import React, {useEffect, useState, type ReactNode} from "react";

import {LangString, ProblemTitle, StateColor, StateString, TimeBeautify} from "@/lib/lib";

import styles from "./SubmissionsList.module.css";

import type {SubmissionWithProgress} from "@/lib/useSubmissionsWebSocket";

interface SubmissionsListProps {
    submissions: SubmissionWithProgress[];
    highlightedIds?: Set<string>;
    canRejudge?: boolean;
    onRejudgeSubmission?: (submissionId: string) => Promise<void>;
    rejudgingId?: string | null;
}

interface VerdictCellProps {
    submission: SubmissionWithProgress;
}

const VerdictCell = ({submission}: VerdictCellProps) => {
  const {state, progress} = submission;

  // State 1 = Saved (in queue, not yet testing)
  if (state === 1 && !progress) {
    return (
      <div className={styles.queueText}>
        <Loader size="xs" />
        <span>В очереди...</span>
      </div>
    );
  }

  // Currently testing (has progress)
  if (progress) {
    const phaseLabels = {
      queued: 'В очереди',
      compiling: 'Компиляция',
      testing: `Тест ${progress.testNumber}`
    };
    return (
      <div className={styles.progressText}>
        <Loader size="xs" />
        <span>{phaseLabels[progress.phase]}</span>
      </div>
    );
  }

  // Final verdict
  const stateString = StateString(state);
  return (
    <Text c={StateColor(state)} fw={500}>
      {stateString === "UK" ? state : stateString}
    </Text>
  );
};

interface SubmissionRowProps {
    submission: SubmissionWithProgress;
    isHighlighted: boolean;
    isNew: boolean;
    canRejudge?: boolean;
    onRejudgeSubmission?: (submissionId: string) => Promise<void>;
    rejudgingId?: string | null;
}

const SubmissionRow = ({
  submission,
  isHighlighted,
  isNew,
  canRejudge,
  onRejudgeSubmission,
  rejudgingId,
}: SubmissionRowProps) => {
  const [mounted, setMounted] = useState(!isNew);

  useEffect(() => {
    if (isNew) {
      // Trigger animation after mount
      const timer = setTimeout(() => setMounted(true), 10);
      return () => clearTimeout(timer);
    }
  }, [isNew]);

  const rowClasses = [
    isHighlighted ? styles.rowHighlight : '',
    isNew && mounted ? styles.rowNew : '',
  ].filter(Boolean).join(' ');

  return (
    <Transition
      mounted={mounted}
      transition="slide-down"
      duration={isNew ? 300 : 0}
      timingFunction="ease-out"
    >
      {(transitionStyles) => (
        <TableTr
          className={rowClasses}
          style={isNew ? transitionStyles : undefined}
        >
          <TableTd ta="center">
            <Text>{TimeBeautify(submission.created_at)}</Text>
          </TableTd>
          <TableTd ta="center">
            <Link href={`/@${submission.username}`} style={{color: 'inherit'}}>
              <Text span td="underline">
                {submission.username}
              </Text>
            </Link>
          </TableTd>
          <TableTd ta="center">
            <Link href={`/contests/${submission.contest_id}/problems/${submission.problem_id}`} style={{color: 'inherit'}}>
              <Text span td="underline">
                {ProblemTitle(submission.position, submission.problem_title)}
              </Text>
            </Link>
          </TableTd>
          <TableTd ta="center">
            <Text>{LangString(submission.language)}</Text>
          </TableTd>
          <TableTd ta="center" className={styles.colVerdict}>
            <VerdictCell submission={submission} />
          </TableTd>
          <TableTd ta="center">
            <Text>{submission.time_stat} ms</Text>
          </TableTd>
          <TableTd ta="center">
            <Text>{submission.memory_stat} КБ</Text>
          </TableTd>
          <TableTd ta="center">
            <Group gap="xs" justify="center" wrap="nowrap">
              <Link href={`/submissions/${submission.id}`} style={{color: 'inherit'}}>
                <Text span td="underline">Посмотреть</Text>
              </Link>
              {canRejudge && (
                <Tooltip label="Перетестировать посылку">
                  <ActionIcon
                    size="sm"
                    variant="subtle"
                    color="blue"
                    loading={rejudgingId === submission.id}
                    onClick={() => onRejudgeSubmission?.(submission.id)}
                  >
                    <IconRefresh size="1rem" />
                  </ActionIcon>
                </Tooltip>
              )}
            </Group>
          </TableTd>
        </TableTr>
      )}
    </Transition>
  );
};

const SubmissionsList = ({
  submissions,
  highlightedIds = new Set(),
  canRejudge,
  onRejudgeSubmission,
  rejudgingId,
}: SubmissionsListProps): ReactNode => {
  return (
    <>
      <TableScrollContainer minWidth={800}>
        <Table className={styles.table}>
          <TableThead>
            <TableTr>
              <TableTh ta="center">Когда</TableTh>
              <TableTh ta="center">Кто</TableTh>
              <TableTh ta="center">Задача</TableTh>
              <TableTh ta="center">Язык</TableTh>
              <TableTh ta="center" className={styles.colVerdict}>Вердикт</TableTh>
              <TableTh ta="center">Время</TableTh>
              <TableTh ta="center">Память</TableTh>
              <TableTh ta="center">Действия</TableTh>
            </TableTr>
          </TableThead>
          <TableTbody>
            {submissions.map((submission) => (
              <SubmissionRow
                key={submission.id}
                submission={submission}
                isHighlighted={highlightedIds.has(submission.id)}
                isNew={submission.isNew ?? false}
                canRejudge={canRejudge}
                onRejudgeSubmission={onRejudgeSubmission}
                rejudgingId={rejudgingId}
              />
            ))}
          </TableTbody>
        </Table>
      </TableScrollContainer>
      {submissions.length === 0 && (
        <Text c="dimmed" ta="center" py="md">
          Посылок нет
        </Text>
      )}
    </>
  );
};

export {SubmissionsList};
