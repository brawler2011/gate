"use client";

import {
    Loader,
    Table,
    TableTbody,
    TableTd,
    TableTh,
    TableThead,
    TableTr,
    Text,
    Transition,
    TableScrollContainer,
} from "@mantine/core";
import { LangString, ProblemTitle, StateColor, StateString, TimeBeautify } from "@/lib/lib";
import Link from "next/link";
import React, { useEffect, useState } from "react";
import type { SubmissionWithProgress } from "@/lib/useSubmissionsWebSocket";
import styles from "./SubmissionsList.module.css";

interface SubmissionsListProps {
    submissions: SubmissionWithProgress[];
    highlightedIds?: Set<string>;
}

interface VerdictCellProps {
    submission: SubmissionWithProgress;
}

const VerdictCell = ({ submission }: VerdictCellProps) => {
    const { state, progress } = submission;

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
        const percentage = progress.totalTests > 0
            ? (progress.testNumber / progress.totalTests) * 100
            : 0;

        return (
            <div className={styles.progressContainer}>
                <div className={styles.progressText}>
                    <Loader size="xs" />
                    <span>Тестируется ({progress.testNumber}/{progress.totalTests})</span>
                </div>
                <div className={styles.progressBar}>
                    <div
                        className={progress.hasFailed ? styles.progressFillError : styles.progressFill}
                        style={{ width: `${percentage}%` }}
                    />
                </div>
            </div>
        );
    }

    // Final verdict
    const stateString = StateString(state, submission.failed_test);
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
}

const SubmissionRow = ({ submission, isHighlighted, isNew }: SubmissionRowProps) => {
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
                        <Link href={`/users/${submission.user_id}`} style={{ color: 'inherit' }}>
                            <Text span td="underline">
                                {submission.username}
                            </Text>
                        </Link>
                    </TableTd>
                    <TableTd ta="center">
                        <Link href={`/contests/${submission.contest_id}/problems/${submission.problem_id}`} style={{ color: 'inherit' }}>
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
                        <Link href={`/submissions/${submission.id}`} style={{ color: 'inherit' }}>
                            <Text span td="underline">Посмотреть</Text>
                        </Link>
                    </TableTd>
                </TableTr>
            )}
        </Transition>
    );
};

const SubmissionsList = ({ submissions, highlightedIds = new Set() }: SubmissionsListProps) => {
    return (
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
                        <TableTh ta="center">Просмотр</TableTh>
                    </TableTr>
                </TableThead>
                <TableTbody>
                    {submissions.map((submission) => (
                        <SubmissionRow
                            key={submission.id}
                            submission={submission}
                            isHighlighted={highlightedIds.has(submission.id)}
                            isNew={submission.isNew ?? false}
                        />
                    ))}
                </TableTbody>
            </Table>
        </TableScrollContainer>
    );
};

export { SubmissionsList };
export type { SubmissionsListProps };
