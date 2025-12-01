'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import type { SubmissionsListItemModel } from '../../contracts/core/v1';

// WebSocket message types from backend
type WebSocketMessageType = 
  | 'submission_created'
  | 'submission_updated'
  | 'testing_started'
  | 'test_completed'
  | 'testing_completed';

// WebSocket event structure from backend (matches Go SubmissionWebSocketEvent)
interface WebSocketEvent {
  message_type: WebSocketMessageType;
  submission?: SubmissionsListItemModel;
  message?: string;
  // Test progress fields
  submission_id?: string;
  test_number?: number;
  total_tests?: number;
  passed?: boolean;
  state?: number;
  // Filter metadata
  contest_id?: string;
  user_id?: string;
  problem_id?: string;
}

// Progress info for a submission being tested
export interface TestProgress {
  testNumber: number;
  totalTests: number;
  hasFailed: boolean;
}

// Extended submission with progress info
export interface SubmissionWithProgress extends SubmissionsListItemModel {
  progress?: TestProgress;
  isNew?: boolean;
  isUpdated?: boolean;
}

export interface UseSubmissionsWebSocketOptions {
  wsUrl: string;
  initialSubmissions: SubmissionsListItemModel[];
  filter: {
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
  highlightedIds: Set<string>;
}

const RECONNECT_DELAYS = [2000, 4000, 8000]; // 2s, 4s, 8s
const MAX_RECONNECT_ATTEMPTS = 3;
const CONNECTION_TIMEOUT = 10000; // 10s
const HIGHLIGHT_DURATION = 2000; // 2s

const isDev = process.env.NODE_ENV === 'development';

function log(...args: unknown[]) {
  if (isDev) {
    console.log('[WS]', ...args);
  }
}

export function useSubmissionsWebSocket({
  wsUrl,
  initialSubmissions,
  filter,
  pageSize,
  enabled,
}: UseSubmissionsWebSocketOptions): UseSubmissionsWebSocketReturn {
  // Calculate filterKey first - used for initialization and change detection
  const filterKey = `${filter.contestId}-${filter.userId}-${filter.problemId}`;
  
  const [submissions, setSubmissions] = useState<SubmissionWithProgress[]>(
    () => initialSubmissions.map(s => ({ ...s }))
  );
  const [connectionError, setConnectionError] = useState(false);
  const [highlightedIds, setHighlightedIds] = useState<Set<string>>(new Set());

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const connectionTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const progressMapRef = useRef<Map<string, TestProgress>>(new Map());
  const mountedRef = useRef(true);
  // Initialize with current filterKey to prevent unnecessary reset on first mount
  const initializedFilterRef = useRef<string>(filterKey);

  // Only reset submissions when filter actually changes (different problem/user/contest)
  // This prevents resetting when parent re-renders with same data
  useEffect(() => {
    if (initializedFilterRef.current !== filterKey) {
      initializedFilterRef.current = filterKey;
      setSubmissions(initialSubmissions.map(s => {
        const existingProgress = progressMapRef.current.get(s.id);
        return existingProgress ? { ...s, progress: existingProgress } : { ...s };
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

  // Build WebSocket URL with filter params
  // Use individual filter values as dependencies to prevent reconnection on every render
  const buildWsUrl = useCallback(() => {
    const url = new URL(wsUrl);
    url.searchParams.set('sortOrder', 'desc');
    if (filter.contestId) url.searchParams.set('contestId', filter.contestId);
    if (filter.userId) url.searchParams.set('userId', filter.userId);
    if (filter.problemId) url.searchParams.set('problemId', filter.problemId);
    return url.toString();
  }, [wsUrl, filter.contestId, filter.userId, filter.problemId]);

  // Handle incoming WebSocket message
  const handleMessage = useCallback((event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data) as WebSocketEvent;

      switch (data.message_type) {
        case 'submission_created': {
          if (!data.submission) return;
          const newSubmission: SubmissionWithProgress = {
            ...data.submission,
            isNew: true,
          };
          
          setSubmissions(prev => {
            // Check if submission already exists (avoid duplicates)
            if (prev.some(s => s.id === newSubmission.id)) {
              return prev;
            }
            // Add to top, remove last to maintain pageSize
            const updated = [newSubmission, ...prev];
            if (updated.length > pageSize) {
              updated.pop();
            }
            return updated;
          });
          addHighlight(data.submission.id);
          break;
        }

        case 'submission_updated': {
          if (!data.submission) return;
          setSubmissions(prev => {
            const index = prev.findIndex(s => s.id === data.submission!.id);
            if (index === -1) {
              // Unknown submission, ignore (might be on different page)
              return prev;
            }
            const updated = [...prev];
            const existingProgress = progressMapRef.current.get(data.submission!.id);
            updated[index] = {
              ...data.submission!,
              progress: existingProgress,
              isUpdated: true,
            };
            return updated;
          });
          addHighlight(data.submission.id);
          // Clear progress when submission is updated (testing complete)
          progressMapRef.current.delete(data.submission.id);
          break;
        }

        case 'testing_started': {
          if (!data.submission_id || !data.total_tests) return;
          const progress: TestProgress = {
            testNumber: 0,
            totalTests: data.total_tests,
            hasFailed: false,
          };
          progressMapRef.current.set(data.submission_id, progress);
          
          setSubmissions(prev => {
            const index = prev.findIndex(s => s.id === data.submission_id);
            if (index === -1) return prev;
            const updated = [...prev];
            updated[index] = { ...updated[index], progress };
            return updated;
          });
          break;
        }

        case 'test_completed': {
          if (!data.submission_id) return;
          const existingProgress = progressMapRef.current.get(data.submission_id);
          const progress: TestProgress = {
            testNumber: data.test_number ?? existingProgress?.testNumber ?? 0,
            totalTests: data.total_tests ?? existingProgress?.totalTests ?? 0,
            hasFailed: existingProgress?.hasFailed || data.passed === false,
          };
          progressMapRef.current.set(data.submission_id, progress);
          
          setSubmissions(prev => {
            const index = prev.findIndex(s => s.id === data.submission_id);
            if (index === -1) return prev;
            const updated = [...prev];
            updated[index] = { ...updated[index], progress };
            return updated;
          });
          break;
        }

        case 'testing_completed': {
          if (!data.submission_id) return;
          // Clear progress - submission_updated will follow with final state
          progressMapRef.current.delete(data.submission_id);
          
          setSubmissions(prev => {
            const index = prev.findIndex(s => s.id === data.submission_id);
            if (index === -1) return prev;
            const updated = [...prev];
            // Update state if provided
            if (data.state !== undefined) {
              updated[index] = {
                ...updated[index],
                state: data.state,
                progress: undefined,
              };
            } else {
              updated[index] = { ...updated[index], progress: undefined };
            }
            return updated;
          });
          addHighlight(data.submission_id);
          break;
        }
      }
    } catch (error) {
      log('Error parsing message:', error);
    }
  }, [pageSize, addHighlight]);

  // Connect to WebSocket
  const connect = useCallback(() => {
    if (!enabled || !wsUrl) {
      if (!wsUrl) {
        console.warn('[WS] WebSocket URL not configured');
      }
      return;
    }

    const fullUrl = buildWsUrl();
    log('Connecting to:', fullUrl);

    try {
      const ws = new WebSocket(fullUrl);
      wsRef.current = ws;

      // Connection timeout
      connectionTimeoutRef.current = setTimeout(() => {
        if (ws.readyState !== WebSocket.OPEN) {
          log('Connection timeout');
          ws.close();
        }
      }, CONNECTION_TIMEOUT);

      ws.onopen = () => {
        log('Connected successfully');
        if (connectionTimeoutRef.current) {
          clearTimeout(connectionTimeoutRef.current);
        }
        reconnectAttemptRef.current = 0;
        setConnectionError(false);
      };

      ws.onmessage = handleMessage;

      ws.onerror = (error) => {
        log('WebSocket error:', error);
      };

      ws.onclose = (event) => {
        log('Disconnected:', event.code, event.reason);
        wsRef.current = null;

        if (connectionTimeoutRef.current) {
          clearTimeout(connectionTimeoutRef.current);
        }

        // Attempt reconnect if not intentionally closed
        if (mountedRef.current && enabled && reconnectAttemptRef.current < MAX_RECONNECT_ATTEMPTS) {
          const delay = RECONNECT_DELAYS[reconnectAttemptRef.current] ?? RECONNECT_DELAYS[RECONNECT_DELAYS.length - 1];
          log(`Reconnecting in ${delay}ms (attempt ${reconnectAttemptRef.current + 1}/${MAX_RECONNECT_ATTEMPTS})`);
          
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptRef.current++;
            connect();
          }, delay);
        } else if (reconnectAttemptRef.current >= MAX_RECONNECT_ATTEMPTS) {
          log('Max reconnect attempts reached');
          setConnectionError(true);
        }
      };
    } catch (error) {
      log('Failed to create WebSocket:', error);
      setConnectionError(true);
    }
  }, [enabled, wsUrl, buildWsUrl, handleMessage]);

  // Setup and cleanup WebSocket connection
  useEffect(() => {
    mountedRef.current = true;

    if (enabled && wsUrl) {
      connect();
    }

    return () => {
      mountedRef.current = false;
      
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (connectionTimeoutRef.current) {
        clearTimeout(connectionTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [enabled, wsUrl, connect]);

  // Reconnect when filter changes
  useEffect(() => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      log('Filter changed, reconnecting...');
      wsRef.current.close();
      reconnectAttemptRef.current = 0;
      // connect() will be called by onclose handler
    }
  }, [filter.contestId, filter.userId, filter.problemId]);

  return {
    submissions,
    connectionError,
    highlightedIds,
  };
}

