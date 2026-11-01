"use client";

import {notifications} from '@mantine/notifications';
import {useEffect, useRef, useState, useCallback, useId} from 'react';

import {api} from './api';
import {connectSubmissions, type SubmissionsConnectionStatus} from './submissionsConnection';

import type {SubmissionsListItemModel} from '@/contracts/core/v1/schema';
import type MessageSubmissionCompleted from '@/contracts/observer/v1/MessageSubmissionCompleted';
import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';

// Progress info for a submission being tested
interface TestProgress {
  phase: 'queued' | 'compiling' | 'testing';
  testNumber?: number;
}

// Extended submission with progress info
export interface SubmissionWithProgress extends MessageSubmissionCompleted {
  organization_login?: string;
  contest_login?: string;
  progress?: TestProgress;
  isNew?: boolean;
}

export interface UseSubmissionsWebSocketOptions {
  wsUrl?: string;
  since?: number;
  initialSubmissions: SubmissionsListItemModel[];
  filter: {
    orgLogin?: string;
    contestLogin?: string;
    contestId?: string;
    userId?: string;
    problemId?: string;
  };
  pageSize: number;
  enabled: boolean;
}

export interface UseSubmissionsWebSocketReturn {
  submissions: SubmissionWithProgress[];
  highlightedIds: Set<string>;
}

const HIGHLIGHT_DURATION = 2000;
const WS_STATUS_DEBOUNCE_MS = 400;
const WS_RESTORED_TOAST_AUTO_CLOSE_MS = 2500;

const asMessage = (event: MessageEvent): SubmissionsMessage | null => {
  try {
    const data = JSON.parse(event.data) as SubmissionsMessage;
    if (!data || typeof data !== 'object') {
      return null;
    }
    if (!('event_type' in data) || !('payload' in data)) {
      return null;
    }
    return data;
  } catch {
    return null;
  }
};

export const useSubmissionsWebSocket = ({
  wsUrl,
  since,
  initialSubmissions,
  filter,
  pageSize,
  enabled,
}: UseSubmissionsWebSocketOptions): UseSubmissionsWebSocketReturn => {
  // Calculate filterKey first - used for initialization and change detection
  const filterKey = `${filter.orgLogin}-${filter.contestLogin}-${filter.contestId}-${filter.userId}-${filter.problemId}`;

  const [submissions, setSubmissions] = useState<SubmissionWithProgress[]>(
    () => initialSubmissions.map(s => ({...s}))
  );
  const [wsStatus, setWsStatus] = useState<SubmissionsConnectionStatus>({phase: 'idle'});
  const wsPhase = wsStatus.phase;
  const reconnectAttempt = 'attempt' in wsStatus ? wsStatus.attempt : 0;
  const reconnectMaxAttempts = 'maxAttempts' in wsStatus ? wsStatus.maxAttempts : 0;
  const [displayWsPhase, setDisplayWsPhase] = useState<SubmissionsConnectionStatus['phase']>('idle');
  const [highlightedIds, setHighlightedIds] = useState<Set<string>>(new Set());
  const sinceRef = useRef(since ?? 0);
  const snapshotGenerationRef = useRef(0);
  const prevDisplayWsPhaseRef = useRef<SubmissionsConnectionStatus['phase']>('idle');
  const wsNotificationId = `submissions-ws-status:${useId()}`;
  const wsNotificationVisibleRef = useRef(false);

  const mountedRef = useRef(true);
  // Initialize with current filterKey to prevent unnecessary reset on first mount
  const initializedFilterRef = useRef<string>(filterKey);

  useEffect(() => {
    sinceRef.current = since ?? 0;
  }, [since]);

  useEffect(() => {
    snapshotGenerationRef.current++;
  }, [filterKey, pageSize]);

  useEffect(() => {
    if (wsPhase === 'fatal') {
      setDisplayWsPhase('fatal');
      return;
    }

    if (wsPhase === 'connected' || wsPhase === 'idle') {
      setDisplayWsPhase('idle');
      return;
    }

    const t = setTimeout(() => {
      setDisplayWsPhase(wsPhase);
    }, WS_STATUS_DEBOUNCE_MS);

    return () => clearTimeout(t);
  }, [wsPhase]);

  useEffect(() => {
    const prevPhase = prevDisplayWsPhaseRef.current;

    const showOrUpdateNotification = (params: {
      title: string;
      message: string;
      color: string;
      autoClose: number | false;
    }) => {
      const notification = {
        id: wsNotificationId,
        ...params,
        onClose: () => {
          wsNotificationVisibleRef.current = false;
        },
      };
      if (wsNotificationVisibleRef.current) {
        notifications.update(notification);
      } else {
        notifications.show(notification);
        wsNotificationVisibleRef.current = true;
      }
    };

    if (!enabled || !wsUrl) {
      notifications.hide(wsNotificationId);
      wsNotificationVisibleRef.current = false;
      prevDisplayWsPhaseRef.current = 'idle';
      return;
    }

    if (displayWsPhase === 'reconnecting') {
      showOrUpdateNotification({
        title: 'Соединение потеряно',
        message: `Переподключение к обновлениям... попытка ${reconnectAttempt}/${reconnectMaxAttempts}`,
        color: 'orange',
        autoClose: false,
      });
    } else if (displayWsPhase === 'fatal') {
      showOrUpdateNotification({
        title: 'Онлайн-обновления недоступны',
        message: 'Не удалось восстановить WebSocket после нескольких попыток. Обновите страницу позже.',
        color: 'red',
        autoClose: false,
      });
    } else if (prevPhase === 'reconnecting' || prevPhase === 'fatal') {
      showOrUpdateNotification({
        title: 'Подключение восстановлено',
        message: 'Онлайн-обновления снова доступны.',
        color: 'green',
        autoClose: WS_RESTORED_TOAST_AUTO_CLOSE_MS,
      });
    }

    prevDisplayWsPhaseRef.current = displayWsPhase;
  }, [displayWsPhase, reconnectAttempt, reconnectMaxAttempts, enabled, wsUrl, wsNotificationId]);

  useEffect(() => {
    return () => {
      notifications.hide(wsNotificationId);
      wsNotificationVisibleRef.current = false;
    };
  }, [wsNotificationId]);

  // Only reset submissions when filter actually changes (different problem/user/contest)
  // This prevents resetting when parent re-renders with same data
  useEffect(() => {
    if (initializedFilterRef.current !== filterKey) {
      initializedFilterRef.current = filterKey;
      setSubmissions(previous => initialSubmissions.map(s => ({
        ...s, progress: previous.find(existing => existing.id === s.id)?.progress,
      })));
    }
  }, [filterKey, initialSubmissions]);

  // Clear highlight after duration
  const addHighlight = useCallback((id: string) => {
    setHighlightedIds(prev => new Set(prev).add(id));
    setTimeout(() => {
      if (mountedRef.current) {
        setHighlightedIds(prev => {
          const next = new Set(prev);
          next.delete(id);
          return next;
        });
      }
    }, HIGHLIGHT_DURATION);
  }, []);

  const updateSubmission = useCallback((
    id: string,
    update: (existing?: SubmissionWithProgress) => SubmissionWithProgress,
    prepend = false,
  ) => {
    setSubmissions(previous => {
      const index = previous.findIndex(s => s.id === id);
      const next = update(previous[index]);
      if (prepend || index === -1) {
        return [next, ...previous.filter(s => s.id !== id)].slice(0, pageSize);
      }
      return previous.map((s, i) => i === index ? next : s);
    });
    addHighlight(id);
  }, [pageSize, addHighlight]);

  // Build WebSocket URL with filter params (without problemId to keep connection alive when switching tasks)
  // Use individual filter values as dependencies to prevent reconnection on every render
  const buildWsUrl = useCallback(() => {
    if (!wsUrl) {
      return '';
    }
    const url = new URL(wsUrl);
    url.searchParams.set('since', String(sinceRef.current));
    url.searchParams.set('sortOrder', 'desc');
    if (filter.contestId) {
      url.searchParams.set('contestId', filter.contestId);
    }
    if (filter.userId) {
      url.searchParams.set('userId', filter.userId);
    }
    return url.toString();
  }, [wsUrl, filter.contestId, filter.userId]);

  const orgLogin = filter.orgLogin;
  const contestLogin = filter.contestLogin;
  const contestId = filter.contestId;
  const userId = filter.userId;
  const problemId = filter.problemId;

  const resyncSnapshot = useCallback(async () => {
    if (!orgLogin || !contestLogin) {
      return;
    }
    const generation = snapshotGenerationRef.current;
    const {data, error} = await api.GET(
      "/organizations/{org_login}/contests/{contest_login}/submissions",
      {params: {
        path: {org_login: orgLogin, contest_login: contestLogin},
        query: {page: 1, pageSize, userId, problemId, sortOrder: 'desc'},
      }},
    );
    if (!mountedRef.current || generation !== snapshotGenerationRef.current) {
      return;
    }
    if (error || !data?.submissions) {
      throw new Error(error?.message || 'snapshot refetch failed');
    }
    sinceRef.current = data.since ?? 0;
    setSubmissions(data.submissions.map(s => ({...s})));
  }, [orgLogin, contestLogin, userId, problemId, pageSize]);

  const toSubmission = useCallback((payload: MessageSubmissionCompleted): SubmissionWithProgress => ({
    ...payload,
    organization_login: orgLogin,
    contest_login: contestLogin,
    state: payload.state,
    score: payload.score,
    penalty: payload.penalty,
    time_stat: payload.time_stat,
    memory_stat: payload.memory_stat,
    failed_test: payload.failed_test,
  }), [orgLogin, contestLogin]);

  // Handle incoming WebSocket message
  const handleMessage = useCallback((event: MessageEvent) => {
    const data = asMessage(event);
    if (!data) {
      return;
    }

    const payload = data.payload;
    if (!payload) {
      return;
    }

    // Client-side filtering: check if the event matches the current task, user, and contest
    if (problemId && payload.problem_id !== problemId) {
      return;
    }
    if (userId && payload.user_id !== userId) {
      return;
    }
    if (contestId && payload.contest_id !== contestId) {
      return;
    }

    switch (data.event_type) {
      case 'submissions.created': {
        const p = data.payload;
        const newSubmission = {...toSubmission(p), isNew: true,
          score: p.frozen ? undefined : 0, penalty: p.frozen ? undefined : 0,
          time_stat: p.frozen ? undefined : 0, memory_stat: p.frozen ? undefined : 0};
        updateSubmission(p.id, () => newSubmission, true);
        break;
      }

      case 'submissions.queued':
      case 'submissions.compiling_started':
      case 'submissions.testing_started':
      case 'submissions.test_started': {
        const p = data.payload;
        const progress: TestProgress = {phase: ({
          'submissions.queued': 'queued',
          'submissions.compiling_started': 'compiling',
          'submissions.testing_started': 'testing',
          'submissions.test_started': 'testing',
        } as const)[data.event_type]};
        if (data.event_type === 'submissions.test_started') {
          progress.testNumber = data.payload.number;
        }
        updateSubmission(p.id, existing => ({
          ...(existing ?? {...toSubmission(p), state: 1, score: 0, penalty: 0,
            time_stat: 0, memory_stat: 0, isNew: true}),
          progress,
        }));
        break;
      }

      case 'submissions.completed': {
        const p = data.payload;
        updateSubmission(p.id, existing => ({...existing, ...toSubmission(p), progress: undefined}));
        break;
      }
    }
  }, [updateSubmission, toSubmission, contestId, userId, problemId]);

  const latest = useRef({handleMessage, resyncSnapshot});
  useEffect(() => {
    latest.current = {handleMessage, resyncSnapshot};
  }, [handleMessage, resyncSnapshot]);

  useEffect(() => {
    mountedRef.current = true;
    if (!enabled || !wsUrl) {
      setWsStatus({phase: 'idle'});
      return;
    }
    const disconnect = connectSubmissions({
      url: buildWsUrl,
      onMessage: event => latest.current.handleMessage(event),
      onResyncRequired: () => latest.current.resyncSnapshot(),
      onStatusChange: setWsStatus,
    });
    return disconnect;
  }, [enabled, wsUrl, buildWsUrl, orgLogin, contestLogin]);

  useEffect(() => () => {
    mountedRef.current = false;
    snapshotGenerationRef.current++;
  }, []);

  return {
    submissions,
    highlightedIds,
  };
};
