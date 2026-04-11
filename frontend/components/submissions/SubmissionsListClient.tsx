'use client';

import React from 'react';
import { SubmissionsList } from './SubmissionsList';
import { useSubmissionsWebSocket } from '@/lib/useSubmissionsWebSocket';
import type { SubmissionsListItemModel } from '@contracts/core/v1';

interface SubmissionsListClientProps {
    initialSubmissions: SubmissionsListItemModel[];
    wsUrl: string;
    since?: number;
    snapshotScope?: 'all' | 'mine';
    filter: {
        contestId?: string;
        userId?: string;
        problemId?: string;
    };
    pageSize: number;
    page: number;
    sortOrder?: string;
}

export function SubmissionsListClient({
    initialSubmissions,
    wsUrl,
    since,
    snapshotScope = 'all',
    filter,
    pageSize,
    page,
    sortOrder,
}: SubmissionsListClientProps) {
    // WS only active on first page with desc sort order (desc is default when not specified)
    const enabled = page === 1 && (sortOrder === 'desc' || sortOrder === undefined);

    const {
        submissions,
        highlightedIds,
    } = useSubmissionsWebSocket({
        wsUrl,
        since,
        initialSubmissions,
        snapshotScope,
        filter,
        pageSize,
        enabled,
    });

    return (
        <>
            <SubmissionsList
                submissions={submissions}
                highlightedIds={highlightedIds}
            />
        </>
    );
}
