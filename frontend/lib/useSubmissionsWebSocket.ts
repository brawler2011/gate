"use client";

import {notifications} from '@mantine/notifications';
import {useEffect, useRef, useState, useCallback, useMemo} from 'react';

import {
  SubmissionsEventType,
  type SubmissionsMessage,
  type MessageSubmissionCreated,
  type MessageSubmissionQueued,
  type MessageSubmissionCompilingStarted,
  type MessageSubmissionTestingStarted,
  type MessageSubmissionTestStarted,
  type MessageSubmissionCompleted,
} from '@/contracts/observer/v1';

import {api, type ApiError} from './api';
import {env} from './env';
import {submissionsWsManager, type WsManagerStatus} from './submissionsWsManager';

import type {ListSubmissionsResponseModel, SubmissionsListItemModel} from '@/contracts/core/v1';

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

type SnapshotParams = {
  orgLogin?: string;
  contestLogin?: string;
  contestId?: string;
  page: number;
  pageSize: number;
  userId?: string;
  problemId?: string;
  state?: number;
  sortOrder?: "asc" | "desc";
  language?: number;
};

type MySubmissionsData = ListSubmissionsResponseModel | null;

const hasSince = (data: MySubmissionsData): data is NonNullable<MySubmissionsData> & { since?: number } => {
  return typeof data === 'object' && data !== null;
};

const toSnapshotParams = (filter: UseSubmissionsWebSocketOptions['filter'], pageSize: number): SnapshotParams | null => {
  if (!filter.orgLogin || !filter.contestLogin) {
    return null;
  }
  return {
    page: 1,
    pageSize,
    orgLogin: filter.orgLogin,
    contestLogin: filter.contestLogin,
    contestId: filter.contestId,
    userId: filter.userId,
    problemId: filter.problemId,
    sortOrder: 'desc',
  };
};

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

const HIGHLIGHT_DURATION = 2000; // 2s
const INVALID_PAYLOAD_LOG_INTERVAL_MS = 3000;
const INVALID_PAYLOAD_WINDOW_MS = 30000;
const WS_STATUS_DEBOUNCE_MS = 400;
const WS_DEBUG = false;
const WS_RESTORED_TOAST_AUTO_CLOSE_MS = 2500;

const isDev = env.isDevelopment();

const log = (...args: unknown[]) => {
  if (isDev && WS_DEBUG) {
    // eslint-disable-next-line no-console
    console.log('[WS]', ...args);
  }
};

const makeWsNotificationId = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `submissions-ws-status:${crypto.randomUUID()}`;
  }
  return `submissions-ws-status:${Date.now()}-${Math.random().toString(36).slice(2)}`;
};

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
    score: 0,
    penalty: 0,
    time_stat: 0,
    memory_stat: 0,
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
    score: payload.score,
    penalty: payload.penalty,
    time_stat: payload.time_stat,
    memory_stat: payload.memory_stat,
    language: payload.language ?? 0,
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
  const [wsPhase, setWsPhase] = useState<WsManagerStatus['phase']>('idle');
  const [displayWsPhase, setDisplayWsPhase] = useState<WsManagerStatus['phase']>('idle');
  const [reconnectAttempt, setReconnectAttempt] = useState(0);
  const [reconnectMaxAttempts, setReconnectMaxAttempts] = useState(0);
  const [highlightedIds, setHighlightedIds] = useState<Set<string>>(new Set());
  const [sinceState, setSinceState] = useState<number | undefined>(since);
  const invalidPayloadCountRef = useRef(0);
  const invalidPayloadWindowStartRef = useRef(0);
  const lastInvalidPayloadLogRef = useRef(0);
  const prevDisplayWsPhaseRef = useRef<WsManagerStatus['phase']>('idle');
  const wsNotificationIdRef = useRef<string>(makeWsNotificationId());
  const wsNotificationVisibleRef = useRef(false);

  const progressMapRef = useRef<Map<string, TestProgress>>(new Map());
  const mountedRef = useRef(true);
  // Initialize with current filterKey to prevent unnecessary reset on first mount
  const initializedFilterRef = useRef<string>(filterKey);

  useEffect(() => {
    setSinceState(since);
  }, [since]);

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
    const wsNotificationId = wsNotificationIdRef.current;

    const showOrUpdateNotification = (params: {
      title: string;
      message: string;
      color: string;
      autoClose: number | false;
    }) => {
      if (wsNotificationVisibleRef.current) {
        notifications.update({
          id: wsNotificationId,
          title: params.title,
          message: params.message,
          color: params.color,
          autoClose: params.autoClose,
          onClose: () => {
            wsNotificationVisibleRef.current = false;
          },
        });
        return;
      }
      notifications.show({
        id: wsNotificationId,
        title: params.title,
        message: params.message,
        color: params.color,
        autoClose: params.autoClose,
        onClose: () => {
          wsNotificationVisibleRef.current = false;
        },
      });
      wsNotificationVisibleRef.current = true;
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
  }, [displayWsPhase, reconnectAttempt, reconnectMaxAttempts, enabled, wsUrl]);

  useEffect(() => {
    const wsNotificationId = wsNotificationIdRef.current;
    return () => {
      notifications.hide(wsNotificationId);
      wsNotificationVisibleRef.current = false;
    };
  }, []);

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
    url.searchParams.set('since', String(sinceState ?? 0));
    url.searchParams.set('sortOrder', 'desc');
    if (filter.contestId) {
      url.searchParams.set('contestId', filter.contestId);
    }
    if (filter.userId) {
      url.searchParams.set('userId', filter.userId);
    }
    return url.toString();
  }, [wsUrl, sinceState, filter.orgLogin, filter.contestLogin, filter.contestId, filter.userId]);

  const connectionKey = useMemo(
    () => `${wsUrl}|${sinceState ?? 0}|${filter.orgLogin ?? ''}|${filter.contestLogin ?? ''}|${filter.contestId ?? ''}|${filter.userId ?? ''}`,
    [wsUrl, sinceState, filter.orgLogin, filter.contestLogin, filter.contestId, filter.userId],
  );

  const orgLogin = filter.orgLogin;
  const contestLogin = filter.contestLogin;
  const contestId = filter.contestId;
  const userId = filter.userId;
  const problemId = filter.problemId;

  const resyncSnapshot = useCallback(async () => {
    const params = toSnapshotParams({orgLogin, contestLogin, contestId, userId, problemId}, pageSize);
    if (!params || !params.orgLogin || !params.contestLogin) {
      return;
    }

    if (snapshotScope === 'mine' && !userId) {
      throw new Error('snapshot scope "mine" requires userId');
    }

    let error: ApiError | null = null;
    let data: ListSubmissionsResponseModel | null = null;

    if (snapshotScope === 'mine') {
      [error, data] = await api.listContestSubmissions({
        orgLogin: params.orgLogin,
        contestLogin: params.contestLogin,
        userId: userId!,
        problemId: params.problemId,
        page: 1,
        pageSize,
        sortOrder: 'desc',
      });
    } else {
      [error, data] = await api.listContestSubmissions({
        orgLogin: params.orgLogin,
        contestLogin: params.contestLogin,
        problemId: params.problemId,
        userId: params.userId,
        state: params.state,
        sortOrder: params.sortOrder,
        language: params.language,
        page: params.page,
        pageSize: params.pageSize,
      });
    }

    if (error || !data || !data.submissions) {
      throw new Error(error?.message || 'snapshot refetch failed');
    }

    setSubmissions(data.submissions.map((s: SubmissionsListItemModel) => ({...s})));
    setSinceState(hasSince(data) ? data.since ?? 0 : 0);
    setFatalConnectionError(false);
    setConnectionError(false);
  }, [orgLogin, contestLogin, contestId, userId, problemId, pageSize, snapshotScope]);

  // Handle incoming WebSocket message
  const handleMessage = useCallback((event: MessageEvent) => {
    const data = asMessage(event);
    if (!data) {
      const now = Date.now();
      if (
        invalidPayloadWindowStartRef.current === 0 ||
        now - invalidPayloadWindowStartRef.current > INVALID_PAYLOAD_WINDOW_MS
      ) {
        invalidPayloadWindowStartRef.current = now;
        invalidPayloadCountRef.current = 0;
      }
      invalidPayloadCountRef.current += 1;

      if (now - lastInvalidPayloadLogRef.current > INVALID_PAYLOAD_LOG_INTERVAL_MS) {
        const raw = typeof event.data === 'string' ? event.data.slice(0, 200) : String(event.data);
        log('Error parsing message: invalid websocket payload', {
          count: invalidPayloadCountRef.current,
          dataType: typeof event.data,
          sample: raw,
          connectionKey,
        });
        lastInvalidPayloadLogRef.current = now;
      }
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
      case SubmissionsEventType.SUBMISSIONS_CREATED: {
        const p = data.payload as MessageSubmissionCreated;
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

      case SubmissionsEventType.SUBMISSIONS_QUEUED:
      case SubmissionsEventType.SUBMISSIONS_COMPILING_STARTED:
      case SubmissionsEventType.SUBMISSIONS_TESTING_STARTED: {
        type QueuedUnion =
          | MessageSubmissionQueued
          | MessageSubmissionCompilingStarted
          | MessageSubmissionTestingStarted;
        const p = data.payload as QueuedUnion;
        let phase: TestProgress['phase'];
        if (data.event_type === SubmissionsEventType.SUBMISSIONS_QUEUED) {
          phase = 'queued';
        } else if (data.event_type === SubmissionsEventType.SUBMISSIONS_COMPILING_STARTED) {
          phase = 'compiling';
        } else {
          phase = 'testing';
        }
        const progress: TestProgress = {phase};
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

      case SubmissionsEventType.SUBMISSIONS_TEST_STARTED: {
        const p = data.payload as MessageSubmissionTestStarted;
        const progress: TestProgress = {
          phase: 'testing',
          testNumber: p.number,
        };
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

      case SubmissionsEventType.SUBMISSIONS_COMPLETED: {
        const p = data.payload as MessageSubmissionCompleted;
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

      default:
        log('Unknown websocket event type:', data.event_type);
    }
  }, [pageSize, addHighlight, connectionKey, contestId, userId, problemId]);

  // Setup and cleanup WebSocket subscription
  const listenerIdRef = useRef<number | null>(null);

  useEffect(() => {
    mountedRef.current = true;

    const id = submissionsWsManager.addListener({
      key: connectionKey,
      url: buildWsUrl(),
      enabled: enabled && Boolean(wsUrl),
      onMessage: handleMessage,
      onResyncRequired: resyncSnapshot,
      onConnectionError: (hasError) => {
        if (mountedRef.current) {
          setConnectionError(hasError);
        }
      },
      onFatalConnectionError: (isFatal) => {
        if (mountedRef.current) {
          setFatalConnectionError(isFatal);
        }
      },
      onStatusChange: (status) => {
        if (!mountedRef.current) {
          return;
        }
        setWsPhase(status.phase);
        if (status.phase === 'reconnecting' || status.phase === 'fatal') {
          setReconnectAttempt(status.attempt);
          setReconnectMaxAttempts(status.maxAttempts);
        } else {
          setReconnectAttempt(0);
          setReconnectMaxAttempts(0);
        }
      },
    });
    listenerIdRef.current = id;

    return () => {
      mountedRef.current = false;
      if (listenerIdRef.current !== null) {
        submissionsWsManager.removeListener(listenerIdRef.current);
        listenerIdRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    if (listenerIdRef.current === null) {
      return;
    }
    submissionsWsManager.updateListener(listenerIdRef.current, {
      key: connectionKey,
      url: buildWsUrl(),
      enabled: enabled && Boolean(wsUrl),
      onMessage: handleMessage,
      onResyncRequired: resyncSnapshot,
      onConnectionError: (hasError) => {
        if (mountedRef.current) {
          setConnectionError(hasError);
        }
      },
      onFatalConnectionError: (isFatal) => {
        if (mountedRef.current) {
          setFatalConnectionError(isFatal);
        }
      },
      onStatusChange: (status) => {
        if (!mountedRef.current) {
          return;
        }
        setWsPhase(status.phase);
        if (status.phase === 'reconnecting' || status.phase === 'fatal') {
          setReconnectAttempt(status.attempt);
          setReconnectMaxAttempts(status.maxAttempts);
        } else {
          setReconnectAttempt(0);
          setReconnectMaxAttempts(0);
        }
      },
    });
  }, [connectionKey, buildWsUrl, enabled, wsUrl, handleMessage, resyncSnapshot]);

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
