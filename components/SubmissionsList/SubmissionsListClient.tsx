'use client';

import React from 'react';
import { SubmissionsList } from './SubmissionsList';
import { useSubmissionsWebSocket } from '@/lib/useSubmissionsWebSocket';
import { notifications } from '@mantine/notifications';
import { useEffect, useRef } from 'react';
import type { SubmissionsListItemModel } from '../../../contracts/core/v1';

interface SubmissionsListClientProps {
    initialSubmissions: SubmissionsListItemModel[];
    wsUrl: string;
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
    filter,
    pageSize,
    page,
    sortOrder,
}: SubmissionsListClientProps) {
    // WS only active on first page with desc sort order (desc is default when not specified)
    const enabled = page === 1 && (sortOrder === 'desc' || sortOrder === undefined);
    const prevErrorRef = useRef(false);

    // Debug logging
    if (process.env.NODE_ENV === 'development') {
        console.log('[SubmissionsListClient]', { wsUrl, page, sortOrder, enabled });
    }

    const { submissions, connectionError, highlightedIds } = useSubmissionsWebSocket({
        wsUrl,
        initialSubmissions,
        filter,
        pageSize,
        enabled,
    });

    // Show toast notification when connection error occurs
    useEffect(() => {
        if (connectionError && !prevErrorRef.current) {
            notifications.show({
                title: 'Соединение потеряно',
                message: 'Не удалось подключиться к серверу обновлений. Данные могут быть неактуальными.',
                color: 'orange',
                autoClose: 5000,
            });
        }
        prevErrorRef.current = connectionError;
    }, [connectionError]);

    return (
        <SubmissionsList
            submissions={submissions}
            highlightedIds={highlightedIds}
        />
    );
}

