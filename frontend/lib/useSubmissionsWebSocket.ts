"use client";

import {notifications} from '@mantine/notifications';
import {useEffect, useRef, useState, useCallback, useMemo, useId} from 'react';

import {api} from './api';
import {submissionsWsManager, type WsManagerStatus} from './submissionsWsManager';

import type {SubmissionsListItemModel} from '@/contracts/core/v1/schema';
import type MessageSubmissionCompleted from '@/contracts/observer/v1/MessageSubmissionCompleted';
import type MessageSubmissionCreated from '@/contracts/observer/v1/MessageSubmissionCreated';
import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';

// Progress info for a submission being tested
interface TestProgress {
  phase: 'queued' | 'compiling' | 'testing';
  testNumber?: number;
}

// Extended submission with progress info
export interface SubmissionWithProgress extends SubmissionsListItemModel {
  progress?: TestProgress;
  isNew?: boolean;
  isUpdated?: boolean;
}

export interface UseSubmissionsWebSocketOptions {
  wsUrl?: string;
  since?: number;
  initialSubmissions: SubmissionsListItemModel[];
  snapshotScope: 'all' | 'mine';
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
  connectionError: boolean;
  fatalConnectionError: boolean;
  wsPhase: WsManagerStatus['phase'];
  displayWsPhase: WsManagerStatus['phase'];
  reconnectAttempt: number;
  reconnectMaxAttempts: number;
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

const buildSubmissionFromCreated = (payload: MessageSubmissionCreated): SubmissionWithProgress => {
  return {
    id: payload.id,
    user_id: payload.user_id ?? '',
    username: payload.username ?? '',
    state: payload.state,
    frozen: payload.frozen,
    score: payload.frozen ? undefined : 0,
    penalty: payload.frozen ? undefined : 0,
    time_stat: payload.frozen ? undefined : 0,
    memory_stat: payload.frozen ? undefined : 0,
    language: payload.language ?? 0,
    problem_id: payload.problem_id ?? '',
    problem_title: payload.problem_title ?? '',
    position: payload.position ?? 0,
    contest_id: payload.contest_id ?? '',
    contest_login: payload.contest_id ?? '',
    contest_title: payload.contest_title ?? '',
    updated_at: payload.created_at ?? new Date().toISOString(),
    created_at: payload.created_at ?? new Date().toISOString(),
    isNew: true,
  };
};

const buildSubmissionFromCompleted = (payload: MessageSubmissionCompleted): SubmissionWithProgress => {
  const now = new Date().toISOString();
  return {
    id: payload.id,
    user_id: payload.user_id ?? '',
    username: payload.username ?? '',
    state: payload.state,
    frozen: payload.frozen,
    score: payload.score,
    penalty: payload.penalty,
    time_stat: payload.time_stat,
    memory_stat: payload.memory_stat,
    language: payload.language ?? 0,
    failed_test: payload.failed_test,
    problem_id: payload.problem_id ?? '',
    problem_title: payload.problem_title ?? '',
    position: payload.position ?? 0,
    contest_id: payload.contest_id ?? '',
    contest_login: payload.contest_id ?? '',
    contest_title: payload.contest_title ?? '',
    updated_at: now,
    created_at: payload.created_at ?? now,
    isUpdated: true,
  };
};

export const useSubmissionsWebSocket = ({
  wsUrl,
  since,
  initialSubmissions,
  snapshotScope,
  filter,
  pageSize,
  enabled,
}: UseSubmissionsWebSocketOptions): UseSubmissionsWebSocketReturn => {
  // Calculate filterKey first - used for initialization and change detection
  const filterKey = `${filter.orgLogin}-${filter.contestLogin}-${filter.contestId}-${filter.userId}-${filter.problemId}`;

  const [submissions, setSubmissions] = useState<SubmissionWithProgress[]>(
    () => initialSubmissions.map(s => ({...s}))
  );
  const [connectionError, setConnectionError] = useState(false);
  const [fatalConnectionError, setFatalConnectionError] = useState(false);
  const [wsStatus, setWsStatus] = useState<WsManagerStatus>({phase: 'idle'});
  const wsPhase = wsStatus.phase;
  const reconnectAttempt = 'attempt' in wsStatus ? wsStatus.attempt : 0;
  const reconnectMaxAttempts = 'maxAttempts' in wsStatus ? wsStatus.maxAttempts : 0;
  const [displayWsPhase, setDisplayWsPhase] = useState<WsManagerStatus['phase']>('idle');
  const [highlightedIds, setHighlightedIds] = useState<Set<string>>(new Set());
  const sinceRef = useRef(since ?? 0);
  const snapshotGenerationRef = useRef(0);
  const prevDisplayWsPhaseRef = useRef<WsManagerStatus['phase']>('idle');
  const wsNotificationId = `submissions-ws-status:${useId()}`;
  const wsNotificationVisibleRef = useRef(false);

  const progressMapRef = useRef<Map<string, TestProgress>>(new Map());
  const mountedRef = useRef(true);
  // Initialize with current filterKey to prevent unnecessary reset on first mount
  const initializedFilterRef = useRef<string>(filterKey);

  useEffect(() => {
    sinceRef.current = since ?? 0;
  }, [since]);

  useEffect(() => {
    snapshotGenerationRef.current++;
  }, [filterKey, snapshotScope, pageSize]);

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
      setSubmissions(initialSubmissions.map(s => {
        const existingProgress = progressMapRef.current.get(s.id);
        return existingProgress ? {...s, progress: existingProgress} : {...s};
      }));
      progressMapRef.current.clear();
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

  const connectionKey = useMemo(
    () => `${wsUrl}|${filter.orgLogin ?? ''}|${filter.contestLogin ?? ''}|${filter.contestId ?? ''}|${filter.userId ?? ''}`,
    [wsUrl, filter.orgLogin, filter.contestLogin, filter.contestId, filter.userId],
  );

  const orgLogin = filter.orgLogin;
  const contestLogin = filter.contestLogin;
  const contestId = filter.contestId;
  const userId = filter.userId;
  const problemId = filter.problemId;

  const resyncSnapshot = useCallback(async () => {
    if (!orgLogin || !contestLogin) {
      return;
    }
    if (snapshotScope === 'mine' && !userId) {
      throw new Error('snapshot scope "mine" requires userId');
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
    setFatalConnectionError(false);
    setConnectionError(false);
  }, [orgLogin, contestLogin, userId, problemId, pageSize, snapshotScope]);

  // Handle incoming WebSocket message
  const handleMessage = useCallback((event: MessageEvent) => {
    const data = asMessage(event);
    if (!data) {
      return;
    }

    const payload = data.payload as { problem_id?: string; user_id?: string; contest_id?: string } | null | undefined;
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
        const newSubmission = buildSubmissionFromCreated(p);
        setSubmissions(prev => {
          const filtered = prev.filter(s => s.id !== newSubmission.id);
          const updated = [newSubmission, ...filtered];
          if (updated.length > pageSize) {
            updated.pop();
          }
          return updated;
        });
        addHighlight(p.id);
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
        progressMapRef.current.set(p.id, progress);

        setSubmissions(prev => {
          const index = prev.findIndex(s => s.id === p.id);
          if (index === -1) {
            const fallback = buildSubmissionFromCreated({
              ...p,
              id: p.id,
              state: 1,
              source: '',
            });
            fallback.progress = progress;
            const updated = [fallback, ...prev];
            if (updated.length > pageSize) {
              updated.pop();
            }
            return updated;
          }
          const updated = [...prev];
          updated[index] = {...updated[index], progress, isUpdated: true};
          return updated;
        });
        addHighlight(p.id);
        break;
      }

      case 'submissions.completed': {
        const p = data.payload;
        progressMapRef.current.delete(p.id);

        setSubmissions(prev => {
          const index = prev.findIndex(s => s.id === p.id);
          const completed = buildSubmissionFromCompleted(p);
          if (index === -1) {
            const updated = [completed, ...prev];
            if (updated.length > pageSize) {
              updated.pop();
            }
            return updated;
          }
          const updated = [...prev];
          updated[index] = {...updated[index], ...completed, progress: undefined, isUpdated: true};
          return updated;
        });
        addHighlight(p.id);
        break;
      }
    }
  }, [pageSize, addHighlight, contestId, userId, problemId]);

  const listenerIdRef = useRef<number | null>(null);
  const listener = useMemo(() => ({
    key: connectionKey,
    url: buildWsUrl,
    enabled: enabled && Boolean(wsUrl),
    onMessage: handleMessage,
    onResyncRequired: resyncSnapshot,
    onConnectionError: (hasError: boolean) => {
      if (mountedRef.current) {
        setConnectionError(hasError);
      }
    },
    onFatalConnectionError: (isFatal: boolean) => {
      if (mountedRef.current) {
        setFatalConnectionError(isFatal);
      }
    },
    onStatusChange: (status: WsManagerStatus) => {
      if (mountedRef.current) {
        setWsStatus(status);
      }
    },
  }), [connectionKey, buildWsUrl, enabled, wsUrl, handleMessage, resyncSnapshot]);

  useEffect(() => {
    mountedRef.current = true;
    if (listenerIdRef.current === null) {
      listenerIdRef.current = submissionsWsManager.addListener(listener);
    } else {
      submissionsWsManager.updateListener(listenerIdRef.current, listener);
    }
  }, [listener]);

  useEffect(() => () => {
    mountedRef.current = false;
    if (listenerIdRef.current !== null) {
      submissionsWsManager.removeListener(listenerIdRef.current);
      listenerIdRef.current = null;
    }
  }, []);

  return {
    submissions,
    connectionError,
    fatalConnectionError,
    wsPhase,
    displayWsPhase,
    reconnectAttempt,
    reconnectMaxAttempts,
    highlightedIds,
  };
};
