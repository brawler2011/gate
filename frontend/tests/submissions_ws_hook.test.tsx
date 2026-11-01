import './setup-dom';

import {notifications} from '@mantine/notifications';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {afterEach, beforeEach, describe, expect, it, jest, spyOn} from 'bun:test';
import {StrictMode} from 'react';

import {SubmissionsProvider} from '@/contexts/SubmissionsProvider';
import {useSubmissions} from '@/hooks/useSubmissions';
import {api} from '@/lib/api';
import {env} from '@/lib/env';
import {SUBMISSIONS_PATH, submissionsParams} from '@/lib/submissions';

import {act, cleanup, renderHook} from './test-utils';
import {flushMicrotasks, TestWebSocket} from './websocket-test-utils';

import type {ListSubmissionsResponseModel, SubmissionsListItemModel} from '@/contracts/core/v1/schema';
import type {SubmissionsSelection} from '@/lib/submissions';
import type {ReactNode} from 'react';

const submission = (id = 'initial'): SubmissionsListItemModel => ({
  id, user_id: 'user', username: 'Alice', contest_id: 'contest', contest_login: 'round', contest_title: 'Round',
  problem_id: 'problem', problem_title: 'A+B', position: 1, language: 20, state: 200,
  created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
});
const data = (...rows: SubmissionsListItemModel[]): ListSubmissionsResponseModel => ({
  submissions: rows, pagination: {page: 1, total: rows.length}, since: 7,
});
const submissionsQueryOptions = (selection: SubmissionsSelection) => api.queryOptions('get', SUBMISSIONS_PATH, submissionsParams(selection));
const selection = (): SubmissionsSelection => ({
  orgLogin: 'school', contestLogin: 'round', contestId: 'contest', page: 1, pageSize: 2, sortOrder: 'desc',
});
const advance = async (ms: number) => act(async () => {
  jest.advanceTimersByTime(ms); await flushMicrotasks();
});
let sequence = 7;
const message = (socket: TestWebSocket, type: string, payload: object = submission()) =>
  socket.message({event_type: `submissions.${type}`, sequence: ++sequence,
    payload: {...(type === 'created' ? {is_new: true} : {previous_state: 1}), ...payload}});

describe('Query-owned submission lists', () => {
  const originalSocket = globalThis.WebSocket;
  let client: QueryClient;
  let fetch: ReturnType<typeof spyOn<typeof globalThis, 'fetch'>>;
  let show: ReturnType<typeof spyOn<typeof notifications, 'show'>>;
  let update: ReturnType<typeof spyOn<typeof notifications, 'update'>>;
  let hide: ReturnType<typeof spyOn<typeof notifications, 'hide'>>;
  let url: ReturnType<typeof spyOn<typeof env, 'getWebSocketUrl'>>;
  beforeEach(() => {
    jest.useFakeTimers();
    sequence = 7;
    client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
    TestWebSocket.instances = [];
    TestWebSocket.failConnections = false;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    fetch = spyOn(globalThis, 'fetch').mockImplementation(Object.assign(async () => Response.json(data(submission())),
      {preconnect: () => {}}));
    show = spyOn(notifications, 'show').mockReturnValue('notice');
    update = spyOn(notifications, 'update').mockReturnValue('notice');
    hide = spyOn(notifications, 'hide').mockReturnValue('notice');
    url = spyOn(env, 'getWebSocketUrl').mockReturnValue('ws://localhost/ws');
  });
  afterEach(async () => {
    cleanup(); client.clear();
    await flushMicrotasks();
    fetch.mockRestore(); show.mockRestore(); update.mockRestore(); hide.mockRestore(); url.mockRestore();
    jest.useRealTimers(); globalThis.WebSocket = originalSocket;
  });
  const Wrapper = ({children}: {children: ReactNode}) => <QueryClientProvider client={client}>
    <SubmissionsProvider contestId="contest" orgLogin="school" contestLogin="round" userId="user">
      {children}
    </SubmissionsProvider>
  </QueryClientProvider>;
  const view = () => renderHook(useSubmissions, {initialProps: selection(), wrapper: Wrapper});
  const open = async () => {
    await act(flushMicrotasks);
    const socket = TestWebSocket.instances.at(-1)!;
    await act(async () => {
      socket.open(); await flushMicrotasks();
    });
    return socket;
  };

  it('passes the full selection to HTTP and scopes the single stream to its contest', async () => {
    renderHook(useSubmissions, {initialProps: {...selection(), userId: 'user', problemId: 'problem',
      state: 200, language: 20, page: 3, sortOrder: 'asc'}, wrapper: Wrapper});
    const socket = await open();
    const request = new URL(String(fetch.mock.calls.at(-1)![0]), 'http://localhost');
    expect(Object.fromEntries(request.searchParams)).toEqual({
      userId: 'user', problemId: 'problem', state: '200', language: '20', page: '3', pageSize: '2', sortOrder: 'asc',
    });
    expect(socket.url).toBe('ws://localhost/ws/submissions?contestId=contest');
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it('changes pages, sorting and filters without replacing the socket', async () => {
    const {result, rerender} = view();
    const socket = await open();
    fetch.mockImplementation(Object.assign(async () => Response.json(data(submission('page-2'))), {preconnect: () => {}}));
    rerender({...selection(), page: 2, sortOrder: 'asc', language: 42, state: 0});
    await act(flushMicrotasks);
    expect(result.current.submissions.map(s => s.id)).toEqual(['page-2']);
    expect(String(fetch.mock.calls.at(-1)![0])).toContain('state=0');
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(socket.closeCalls).toBe(0);
  });

  it('updates all four progress variants in existing cache rows without HTTP or reordering', async () => {
    const {result} = view(); const socket = await open();
    const calls = fetch.mock.calls.length;
    for (const [type, progress] of [
      ['queued', {phase: 'queued'}], ['compiling_started', {phase: 'compiling'}],
      ['testing_started', {phase: 'testing'}], ['test_started', {phase: 'testing', testNumber: 4}],
    ] as const) {
      await act(async () => message(socket, type, {...submission(), number: 4}));
      expect(result.current.submissions[0].progress).toEqual(progress);
      expect(result.current.submissions[0].state).toBe(200);
    }
    expect(fetch).toHaveBeenCalledTimes(calls);
  });

  it('inserts and completes rows without HTTP, preserving totals, zero and null values', async () => {
    const {result} = view(); const socket = await open(); const calls = fetch.mock.calls.length;
    await act(async () => {
      message(socket, 'created', {...submission('second'), state: 1, created_at: '2026-01-01T00:01:00Z'});
      message(socket, 'created', {...submission('third'), state: 1, created_at: '2026-01-01T00:02:00Z'});
      message(socket, 'completed', {...submission('third'), created_at: '2026-01-01T00:02:00Z', score: 0, failed_test: null});
    });
    expect(fetch).toHaveBeenCalledTimes(calls);
    expect(result.current.submissions.map(s => s.id)).toEqual(['third', 'second']);
    expect(result.current.data?.pagination.total).toBe(3);
    expect(result.current.submissions[0]).toMatchObject({state: 200, score: 0, failed_test: null});
  });

  it('clears hidden results when the server marks a row frozen', async () => {
    const {result} = view(); const socket = await open();
    await act(async () => message(socket, 'completed', {...submission(), score: 100, time_stat: 12}));
    const {state: _state, ...metadata} = submission();
    await act(async () => message(socket, 'completed', {...metadata, frozen: true}));
    expect(result.current.submissions[0]).toMatchObject({frozen: true});
    expect(result.current.submissions[0].state).toBeUndefined();
    expect(result.current.submissions[0].score).toBeUndefined();
    expect(result.current.submissions[0].time_stat).toBeUndefined();
  });

  it('ignores repeated and older events using the snapshot cursor', async () => {
    const {result} = view(); const socket = await open();
    const event = {event_type: 'submissions.created', sequence: 8, payload: {...submission('new'), is_new: true}};
    await act(async () => {
      socket.message(event); socket.message(event);
      socket.message({...event, sequence: 7, payload: {...event.payload, id: 'obsolete'}});
    });
    expect(result.current.data?.pagination.total).toBe(2);
    expect(result.current.submissions.map(s => s.id)).toEqual(['new', 'initial']);
  });

  it('does not synthesize unknown progress rows or accept foreign and malformed events', async () => {
    const {result} = view(); const socket = await open(); const calls = fetch.mock.calls.length;
    await act(async () => {
      message(socket, 'queued', submission('unknown'));
      message(socket, 'created', {...submission('foreign'), contest_id: 'other'});
      socket.message({event_type: 'submissions.completed', payload: null});
      socket.onmessage?.({data: 'not JSON'} as MessageEvent);
      message(socket, 'unknown');
    });
    await advance(100);
    expect(result.current.submissions.map(s => s.id)).toEqual(['initial']);
    expect(fetch).toHaveBeenCalledTimes(calls);
  });

  it('keeps multiple query views on one connection and patches only existing rows', async () => {
    const {result} = renderHook(() => ({
      first: useSubmissions(selection()), second: useSubmissions({...selection(), page: 2}),
    }), {wrapper: Wrapper});
    const socket = await open();
    await act(async () => message(socket, 'test_started', {...submission(), number: 2}));
    expect(result.current.first.submissions[0].progress?.testNumber).toBe(2);
    expect(result.current.second.submissions[0].progress?.testNumber).toBe(2);
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it('restarts a single highlight timer for repeated events', async () => {
    const {result} = view(); const socket = await open();
    await act(async () => message(socket, 'queued'));
    await advance(1000);
    await act(async () => message(socket, 'test_started', {...submission(), number: 1}));
    await advance(1000);
    expect(result.current.highlightedIds.has('initial')).toBe(true);
    await advance(1000);
    expect(result.current.highlightedIds.size).toBe(0);
  });

  it('debounces reconnect notices and refreshes data after reconnection', async () => {
    const {result} = view(); const socket = await open();
    await act(async () => socket.disconnect()); await advance(399);
    expect(show).not.toHaveBeenCalled(); await advance(1);
    expect(show).toHaveBeenLastCalledWith(expect.objectContaining({color: 'orange', autoClose: false}));
    fetch.mockImplementation(Object.assign(async () => Response.json(data(submission('recovered'))), {preconnect: () => {}}));
    await advance(1600); await open();
    expect(result.current.submissions[0].id).toBe('recovered');
    expect(update).toHaveBeenLastCalledWith(expect.objectContaining({color: 'green', autoClose: 2500}));
  });

  it('preserves cached rows on HTTP failure without killing the socket', async () => {
    const {result} = view(); const socket = await open();
    fetch.mockImplementation(Object.assign(async () => Response.json({message: 'unavailable'}, {status: 500}), {preconnect: () => {}}));
    await act(async () => socket.disconnect()); await advance(2000); await open();
    expect(result.current.submissions[0].id).toBe('initial');
    expect(result.current.error).toBeTruthy();
    expect(TestWebSocket.instances.at(-1)!.readyState).toBe(TestWebSocket.OPEN);
  });

  it('ignores an obsolete request when the selected problem changes', async () => {
    const {result, rerender} = view(); await open();
    let finish!: (response: Response) => void;
    fetch.mockImplementationOnce(Object.assign(() => new Promise<Response>(resolve => {
      finish = resolve;
    }), {preconnect: () => {}}));
    await act(async () => {
      void client.invalidateQueries(); await flushMicrotasks();
    });
    fetch.mockImplementation(Object.assign(async () => Response.json(data(submission('latest'))), {preconnect: () => {}}));
    rerender({...selection(), problemId: 'other'});
    await act(flushMicrotasks);
    expect(result.current.submissions[0].id).toBe('latest');
    await act(async () => {
      finish(Response.json(data(submission('obsolete')))); await flushMicrotasks();
    });
    expect(result.current.submissions[0].id).toBe('latest');
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it('cancels retries and pending refreshes on unmount', async () => {
    const {unmount} = view(); const socket = await open(); const calls = fetch.mock.calls.length;
    await act(async () => {
      message(socket, 'completed'); socket.disconnect();
    });
    unmount(); await advance(30000);
    expect(fetch).toHaveBeenCalledTimes(calls);
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(hide).toHaveBeenCalled();
  });

  it('keeps one live connection through StrictMode replay', async () => {
    const wrapper = ({children}: {children: ReactNode}) => <StrictMode><Wrapper>{children}</Wrapper></StrictMode>;
    const result = renderHook(useSubmissions, {initialProps: selection(), wrapper});
    await open();
    expect(TestWebSocket.instances.filter(s => s.readyState !== TestWebSocket.CLOSED)).toHaveLength(1);
    result.unmount(); await act(flushMicrotasks);
    expect(TestWebSocket.instances.every(s => s.readyState === TestWebSocket.CLOSED)).toBe(true);
  });

  it('moves a rejudged submission between state filters without HTTP or inflating totals', async () => {
    const {result} = renderHook(() => ({
      filtered: useSubmissions({...selection(), state: 200}), all: useSubmissions(selection()),
    }), {wrapper: Wrapper});
    const socket = await open(); const calls = fetch.mock.calls.length;
    await act(async () => message(socket, 'created', {...submission(), state: 1, is_new: false, previous_state: 200}));
    expect(result.current.filtered.submissions).toHaveLength(0);
    expect(result.current.filtered.data?.pagination.total).toBe(0);
    expect(result.current.all.data?.pagination.total).toBe(1);
    await act(async () => message(socket, 'completed', {...submission(), previous_state: 1}));
    expect(result.current.filtered.submissions).toHaveLength(1);
    expect(result.current.filtered.data?.pagination.total).toBe(1);
    expect(fetch).toHaveBeenCalledTimes(calls);
  });

  it('reapplies an event received while an HTTP snapshot is loading', async () => {
    const {result} = view(); const socket = await open();
    let finish!: (response: Response) => void;
    fetch.mockImplementationOnce(Object.assign(() => new Promise<Response>(resolve => {
      finish = resolve;
    }), {preconnect: () => {}}));
    await act(async () => {
      void client.invalidateQueries(); await flushMicrotasks();
    });
    const calls = fetch.mock.calls.length;
    await act(async () => message(socket, 'completed', {...submission(), score: 42}));
    await act(async () => {
      finish(Response.json(data(submission()))); await flushMicrotasks();
    });
    expect(result.current.submissions[0].score).toBe(42);
    expect(fetch).toHaveBeenCalledTimes(calls);
  });

  it('keeps numbered pages and ascending lists stable when new rows arrive', async () => {
    const {result} = renderHook(useSubmissions, {initialProps: {...selection(), page: 2}, wrapper: Wrapper});
    const socket = await open(); const calls = fetch.mock.calls.length;
    await act(async () => message(socket, 'created', submission('new')));
    expect(result.current.submissions.map(s => s.id)).toEqual(['initial']);
    expect(fetch).toHaveBeenCalledTimes(calls);
  });

  it('cancels a request started before connection instead of restoring its stale result', async () => {
    let finish!: (response: Response) => void;
    fetch.mockImplementationOnce(Object.assign(() => new Promise<Response>(resolve => {
      finish = resolve;
    }), {preconnect: () => {}}));
    const {result} = view();
    fetch.mockImplementation(Object.assign(async () => Response.json(data(submission('fresh'))), {preconnect: () => {}}));
    await open();
    expect(result.current.submissions[0].id).toBe('fresh');
    await act(async () => {
      finish(Response.json(data(submission('stale')))); await flushMicrotasks();
    });
    expect(result.current.submissions[0].id).toBe('fresh');
  });

  it('reports fatal after bounded connection failures', async () => {
    TestWebSocket.failConnections = true; view();
    await act(flushMicrotasks);
    for (const delay of [2000, 4000, 8000]) {
      await advance(delay);
    }
    expect(update).toHaveBeenLastCalledWith(expect.objectContaining({color: 'red', autoClose: false}));
  });

  it('has one cache entry for each complete selection', async () => {
    const base = selection(); view(); await open();
    await client.fetchQuery(submissionsQueryOptions({...base, language: 42}));
    expect(client.getQueryData(submissionsQueryOptions(base).queryKey)).toBeTruthy();
    expect(client.getQueryData(submissionsQueryOptions({...base, language: 42}).queryKey)).toBeTruthy();
    expect(client.getQueryCache().getAll()).toHaveLength(2);
  });
});
