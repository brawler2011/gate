import "./setup-dom";

import {notifications} from "@mantine/notifications";
import {afterEach, beforeEach, describe, expect, it, jest, spyOn} from "bun:test";
import {StrictMode} from "react";

import {useSubmissionsWebSocket} from "@/lib/useSubmissionsWebSocket";

import {act, cleanup, renderHook} from "./test-utils";
import {flushMicrotasks, TestWebSocket} from "./websocket-test-utils";

import type {SubmissionsListItemModel} from "@/contracts/core/v1/schema";
import type {UseSubmissionsWebSocketOptions} from "@/lib/useSubmissionsWebSocket";

const submission = (id: string): SubmissionsListItemModel => ({
  id, user_id: "user", username: "Alice", contest_id: "contest", contest_login: "round", contest_title: "Round",
  problem_id: "problem", problem_title: "A+B", position: 1, language: 20, state: 200,
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
});
const options = (): UseSubmissionsWebSocketOptions => ({
  wsUrl: "ws://localhost/ws/submissions", since: 1, initialSubmissions: [submission("initial")],
  snapshotScope: "all", pageSize: 2, enabled: true,
  filter: {orgLogin: "school", contestLogin: "round", contestId: "contest", userId: "user", problemId: "problem"},
});
const message = (socket: TestWebSocket, type: string, payload: object) => {
  socket.message({event_type: `submissions.${type}`, payload});
};
const advance = async (milliseconds: number) => {
  await act(async () => {
    jest.advanceTimersByTime(milliseconds);
    await flushMicrotasks();
  });
};

describe("Submission WebSocket hook", () => {
  const originalSocket = globalThis.WebSocket;
  let fetch: ReturnType<typeof spyOn<typeof globalThis, "fetch">>;
  let show: ReturnType<typeof spyOn<typeof notifications, "show">>;
  let update: ReturnType<typeof spyOn<typeof notifications, "update">>;
  let hide: ReturnType<typeof spyOn<typeof notifications, "hide">>;

  beforeEach(() => {
    jest.useFakeTimers();
    TestWebSocket.instances = [];
    TestWebSocket.failConnections = false;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      async () => Response.json({submissions: [submission("initial")], since: 7}), {preconnect: () => {}},
    ));
    show = spyOn(notifications, "show").mockReturnValue("notice");
    update = spyOn(notifications, "update").mockReturnValue("notice");
    hide = spyOn(notifications, "hide").mockReturnValue("notice");
  });
  afterEach(async () => {
    cleanup();
    await flushMicrotasks();
    fetch.mockRestore();
    show.mockRestore();
    update.mockRestore();
    hide.mockRestore();
    jest.useRealTimers();
    globalThis.WebSocket = originalSocket;
  });

  const open = async () => {
    await act(flushMicrotasks);
    const socket = TestWebSocket.instances.at(-1)!;
    await act(async () => socket.open());
    return socket;
  };

  it("loads the filtered snapshot and connects with its replay cursor", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    const request = new URL(String(fetch.mock.calls[0][0]), "http://localhost");
    expect(request.pathname).toBe("/api/organizations/school/contests/round/submissions");
    expect(Object.fromEntries(request.searchParams)).toEqual({
      page: "1", pageSize: "2", userId: "user", problemId: "problem", sortOrder: "desc",
    });
    const stream = new URL(socket.url);
    expect(Object.fromEntries(stream.searchParams)).toEqual({since: "7", sortOrder: "desc", contestId: "contest", userId: "user"});
    expect(result.current.submissions.map(item => item.id)).toEqual(["initial"]);
    expect(result.current.wsPhase).toBe("connected");
    expect(show).not.toHaveBeenCalled();
  });

  it("deduplicates created submissions, bounds the page and expires highlights", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    await act(async () => {
      message(socket, "created", submission("second"));
      message(socket, "created", submission("third"));
      message(socket, "created", submission("second"));
    });
    expect(result.current.submissions.map(item => item.id)).toEqual(["second", "third"]);
    expect(result.current.submissions[0].isNew).toBe(true);
    expect(result.current.highlightedIds.has("second")).toBe(true);
    await advance(2000);
    expect(result.current.highlightedIds.size).toBe(0);
  });

  it("applies every progress event and completes in place without losing zero results", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    for (const [type, progress] of [
      ["queued", {phase: "queued"}], ["compiling_started", {phase: "compiling"}],
      ["testing_started", {phase: "testing"}], ["test_started", {phase: "testing", testNumber: 4}],
    ] as const) {
      await act(async () => message(socket, type, {...submission("initial"), number: 4}));
      expect(result.current.submissions[0].progress).toEqual(progress);
      expect(result.current.submissions[0].state).toBe(200);
    }
    await act(async () => {
      message(socket, "created", submission("newer"));
      message(socket, "completed", {...submission("initial"), state: 0, score: 0, failed_test: null});
    });
    expect(result.current.submissions.map(item => item.id)).toEqual(["newer", "initial"]);
    expect(result.current.submissions[1]).toMatchObject({state: 0, score: 0, failed_test: null, isUpdated: true});
    expect(result.current.submissions[1].progress).toBeUndefined();
  });

  it("inserts missing progress/completed submissions and hides frozen results", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    await act(async () => message(socket, "test_started", {...submission("missing"), number: 2}));
    expect(result.current.submissions[0]).toMatchObject({id: "missing", state: 1, progress: {phase: "testing", testNumber: 2}});
    await act(async () => message(socket, "completed", {...submission("completed"), score: 42}));
    expect(result.current.submissions.map(item => item.id)).toEqual(["completed", "missing"]);
    await act(async () => message(socket, "completed", {
      id: "completed", user_id: "user", contest_id: "contest", problem_id: "problem", frozen: true,
    }));
    expect(result.current.submissions[0].frozen).toBe(true);
    expect(result.current.submissions[0].score).toBeUndefined();
    expect(result.current.submissions[0].state).toBeUndefined();
  });

  it("ignores malformed messages and events outside the current filter", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    const before = result.current.submissions;
    await act(async () => {
      socket.onmessage?.({data: "not JSON"} as MessageEvent);
      socket.message({event_type: "submissions.completed", payload: null});
      message(socket, "unknown", submission("unknown"));
      for (const key of ["problem_id", "contest_id", "user_id"]) {
        message(socket, "created", {...submission("other"), [key]: "other"});
      }
    });
    expect(result.current.submissions).toBe(before);
    expect(result.current.highlightedIds.size).toBe(0);
  });

  it("preserves updates on rerender and switches problem filters without reopening the stream", async () => {
    const initial = options();
    const {result, rerender} = renderHook(useSubmissionsWebSocket, {initialProps: initial});
    const socket = await open();
    await act(async () => message(socket, "created", submission("live")));
    rerender({...initial, initialSubmissions: [submission("initial")]});
    expect(result.current.submissions[0].id).toBe("live");
    rerender({...initial, filter: {...initial.filter, problemId: "other"},
      initialSubmissions: [{...submission("other"), problem_id: "other"}]});
    expect(result.current.submissions.map(item => item.id)).toEqual(["other"]);
    await act(async () => {
      message(socket, "created", submission("old-problem"));
      message(socket, "created", {...submission("new-problem"), problem_id: "other"});
    });
    expect(result.current.submissions.map(item => item.id)).toEqual(["new-problem", "other"]);
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("debounces the disconnect notice and reports snapshot recovery", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    await act(async () => socket.disconnect());
    await advance(399);
    expect(show).not.toHaveBeenCalled();
    await advance(1);
    expect(show).toHaveBeenLastCalledWith(expect.objectContaining({color: "orange", autoClose: false}));
    expect(result.current.reconnectAttempt).toBe(1);
    fetch.mockImplementation(Object.assign(async () => Response.json({submissions: [submission("recovered")], since: 9}),
      {preconnect: () => {}}));
    await advance(1600);
    expect(result.current.submissions[0].id).toBe("recovered");
    expect(TestWebSocket.instances.at(-1)!.url).toContain("since=9");
    await act(async () => TestWebSocket.instances.at(-1)!.open());
    expect(result.current.displayWsPhase).toBe("idle");
    expect(update).toHaveBeenLastCalledWith(expect.objectContaining({color: "green", autoClose: 2500}));
  });

  it("retains data and reports fatal status when all recovery snapshots fail", async () => {
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: options()});
    const socket = await open();
    fetch.mockImplementation(Object.assign(async () => Response.json({message: "unavailable"}, {status: 500}),
      {preconnect: () => {}}));
    await act(async () => socket.disconnect());
    for (const delay of [2000, 4000, 8000]) {
      await advance(delay);
    }
    expect(result.current.submissions[0].id).toBe("initial");
    expect(result.current.connectionError).toBe(true);
    expect(result.current.fatalConnectionError).toBe(true);
    expect(result.current.wsPhase).toBe("fatal");
    expect(update).toHaveBeenLastCalledWith(expect.objectContaining({color: "red", autoClose: false}));
  });

  it("does not fetch or connect while disabled and cancels retries on unmount", async () => {
    const {rerender, unmount} = renderHook(useSubmissionsWebSocket, {initialProps: {...options(), enabled: false}});
    await act(flushMicrotasks);
    expect(fetch).not.toHaveBeenCalled();
    rerender(options());
    const socket = await open();
    await act(async () => socket.disconnect());
    unmount();
    await advance(30000);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(hide).toHaveBeenCalled();
  });

  it("requires a user for the personal snapshot", async () => {
    const props = options();
    const {result} = renderHook(useSubmissionsWebSocket, {initialProps: {
      ...props, snapshotScope: "mine", filter: {...props.filter, userId: undefined},
    }});
    await act(flushMicrotasks);
    expect(fetch).not.toHaveBeenCalled();
    expect(TestWebSocket.instances).toHaveLength(0);
    expect(result.current.connectionError).toBe(true);
  });

  it("keeps one live stream through StrictMode replay and closes it on unmount", async () => {
    const view = renderHook(useSubmissionsWebSocket, {initialProps: options(), wrapper: StrictMode});
    await open();
    expect(TestWebSocket.instances.filter(socket => socket.readyState !== TestWebSocket.CLOSED)).toHaveLength(1);
    view.unmount();
    await act(flushMicrotasks);
    expect(TestWebSocket.instances.every(socket => socket.readyState === TestWebSocket.CLOSED)).toBe(true);
  });
});
