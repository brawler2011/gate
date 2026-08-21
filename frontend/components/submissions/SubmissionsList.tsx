"use client";

import {
  ActionIcon,
  Group,
  Loader,
  Menu,
  MenuDropdown,
  MenuItem,
  MenuTarget,
  Table,
  TableScrollContainer,
  TableTbody,
  TableTd,
  TableTh,
  TableThead,
  TableTr,
  Text,
  Tooltip,
  Transition,
} from "@mantine/core";
import {
  IconBan,
  IconDots,
  IconLockOpen,
  IconRefresh,
} from "@tabler/icons-react";
import Link from "next/link";
import React, {useEffect, useState, type ReactNode} from "react";

import {
  LangString,
  numberToLetters,
  ProblemTitle,
  ShortVerdictString,
  StateColor,
  StateString,
  TimeBeautify,
} from "@/lib/lib";

import {SubmissionDetailsModal} from "./SubmissionDetailsModal";
import {
  SubmissionSanctionsModal,
  type SanctionActionType,
} from "./SubmissionSanctionsModal";
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
    onOpenDetails?: () => void;
}

const VerdictCell = ({submission, onOpenDetails}: VerdictCellProps) => {
  const {state, progress, failed_test} = submission;

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

interface SubmissionRowProps {
  submission: SubmissionWithProgress;
  isHighlighted: boolean;
  isNew: boolean;
  canRejudge?: boolean;
  onRejudgeSubmission?: (submissionId: string) => Promise<void>;
  rejudgingId?: string | null;
  onOpenDetails: (submissionId: string) => void;
  onOpenSanctionModal?: (action: SanctionActionType, sub: SubmissionWithProgress) => void;
}

const SubmissionRow = ({
  submission,
  isHighlighted,
  isNew,
  canRejudge,
  onRejudgeSubmission,
  rejudgingId,
  onOpenDetails,
  onOpenSanctionModal,
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
            <Link
              href={
                submission.organization_login && submission.contest_login
                  ? `/${submission.organization_login}/${submission.contest_login}/${numberToLetters(submission.position)}`
                  : `/${submission.contest_login}/${numberToLetters(submission.position)}`
              }
              style={{color: 'inherit'}}
            >
              <Text span td="underline">
                {ProblemTitle(submission.position, submission.problem_title)}
              </Text>
            </Link>
          </TableTd>
          <TableTd ta="center">
            <Text>{LangString(submission.language)}</Text>
          </TableTd>
          <TableTd ta="center" className={styles.colVerdict}>
            <VerdictCell submission={submission} onOpenDetails={() => onOpenDetails(submission.id)} />
          </TableTd>
          <TableTd ta="center">
            <Text>{submission.time_stat} ms</Text>
          </TableTd>
          <TableTd ta="center">
            <Text>{submission.memory_stat} КБ</Text>
          </TableTd>
          <TableTd ta="center">
            <Group gap="xs" justify="center" wrap="nowrap">
              <Text
                span
                td="underline"
                style={{cursor: "pointer"}}
                onClick={() => onOpenDetails(submission.id)}
              >
                Детали
              </Text>
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
              {canRejudge && submission.organization_login && submission.contest_login && (
                <Menu shadow="md" width={220} position="bottom-end">
                  <MenuTarget>
                    <Tooltip label="Модерация и санкции">
                      <ActionIcon size="sm" variant="subtle" color="gray">
                        <IconDots size="1rem" />
                      </ActionIcon>
                    </Tooltip>
                  </MenuTarget>
                  <MenuDropdown>
                    {submission.state === 300 ? (
                      <MenuItem
                        color="blue"
                        leftSection={<IconLockOpen size={14} />}
                        onClick={() => onOpenSanctionModal?.("unblock_submission", submission)}
                      >
                        Разблокировать посылку
                      </MenuItem>
                    ) : (
                      <MenuItem
                        color="red"
                        leftSection={<IconBan size={14} />}
                        onClick={() => onOpenSanctionModal?.("block_submission", submission)}
                      >
                        Заблокировать посылку (DQ)
                      </MenuItem>
                    )}
                    <MenuItem
                      color="red"
                      leftSection={<IconBan size={14} />}
                      onClick={() => onOpenSanctionModal?.("block_problem", submission)}
                    >
                      Заблокировать задачу
                    </MenuItem>
                    <MenuItem
                      color="teal"
                      leftSection={<IconLockOpen size={14} />}
                      onClick={() => onOpenSanctionModal?.("unblock_problem", submission)}
                    >
                      Разблокировать задачу
                    </MenuItem>
                  </MenuDropdown>
                </Menu>
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
  const [selectedSubmissionId, setSelectedSubmissionId] = useState<string | null>(null);
  const [sanctionState, setSanctionState] = useState<{
    actionType: SanctionActionType;
    submission: SubmissionWithProgress;
  } | null>(null);

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
                onOpenDetails={(id) => setSelectedSubmissionId(id)}
                onOpenSanctionModal={(actionType, sub) =>
                  setSanctionState({actionType, submission: sub})
                }
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

      <SubmissionDetailsModal
        submissionId={selectedSubmissionId}
        opened={Boolean(selectedSubmissionId)}
        onClose={() => setSelectedSubmissionId(null)}
        canRejudge={canRejudge}
        onRejudge={onRejudgeSubmission}
        isRejudging={Boolean(rejudgingId && rejudgingId === selectedSubmissionId)}
      />

      <SubmissionSanctionsModal
        actionType={sanctionState?.actionType ?? null}
        onClose={() => setSanctionState(null)}
        orgLogin={sanctionState?.submission.organization_login}
        contestLogin={sanctionState?.submission.contest_login}
        submissionId={sanctionState?.submission.id}
        userId={sanctionState?.submission.user_id}
        problemId={sanctionState?.submission.problem_id}
        username={sanctionState?.submission.username}
        problemTitle={sanctionState?.submission.problem_title}
      />
    </>
  );
};

export {SubmissionsList};
