import {afterEach, beforeEach, describe, expect, it, jest, mock} from "bun:test";

import {submissionsWsManager} from "@/lib/submissionsWsManager";

import {flushMicrotasks, TestWebSocket} from "./websocket-test-utils";

type Listener = Parameters<typeof submissionsWsManager.addListener>[0];
const listener = (changes: Partial<Listener> = {}): Listener => ({
  key: "contest", url: () => "ws://localhost/ws/submissions?contestId=contest", enabled: true,
  onMessage: mock(() => {}), onConnectionError: mock(() => {}), onFatalConnectionError: mock(() => {}),
  onStatusChange: mock(() => {}), onResyncRequired: mock(async () => {}), ...changes,
});

describe("WebSocket subscription lifecycle", () => {
  const originalSocket = globalThis.WebSocket;
  let ids: number[];
  const add = (params: Listener) => {
    const id = submissionsWsManager.addListener(params);
    ids.push(id);
    return id;
  };

  beforeEach(() => {
    ids = [];
    TestWebSocket.instances = [];
    TestWebSocket.failConnections = false;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    jest.useFakeTimers();
  });
  afterEach(async () => {
    ids.forEach(id => submissionsWsManager.removeListener(id));
    await flushMicrotasks();
    jest.useRealTimers();
    globalThis.WebSocket = originalSocket;
  });

  it("loads a snapshot before opening the stream", async () => {
    let finish!: () => void;
    const pending = new Promise<void>(resolve => {
      finish = resolve;
    });
    const params = listener({onResyncRequired: mock(() => pending)});
    add(params);
    expect(params.onResyncRequired).toHaveBeenCalledTimes(1);
    expect(TestWebSocket.instances).toHaveLength(0);
    finish();
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(TestWebSocket.instances[0].url).toBe(params.url());
    TestWebSocket.instances[0].open();
    expect(params.onStatusChange).toHaveBeenLastCalledWith({phase: "connected"});
  });

  it("opens only when enabled and closes when disabled", async () => {
    const params = listener({enabled: false});
    const id = add(params);
    await flushMicrotasks();
    expect(params.onResyncRequired).not.toHaveBeenCalled();
    expect(TestWebSocket.instances).toHaveLength(0);
    submissionsWsManager.updateListener(id, {...params, enabled: true});
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(1);
    const socket = TestWebSocket.instances[0];
    socket.open();
    submissionsWsManager.updateListener(id, params);
    await flushMicrotasks();
    expect(socket.closeCalls).toBe(1);
    jest.advanceTimersByTime(30000);
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it("shares the stream between listeners and closes only after the last removal", async () => {
    const first = listener();
    const firstId = add(first);
    await flushMicrotasks();
    const socket = TestWebSocket.instances[0];
    socket.open();
    const second = listener();
    const secondId = add(second);
    socket.message({event_type: "submissions.queued"});
    expect(first.onMessage).toHaveBeenCalledTimes(1);
    expect(second.onMessage).toHaveBeenCalledTimes(1);
    expect(second.onResyncRequired).not.toHaveBeenCalled();
    expect(second.onStatusChange).toHaveBeenLastCalledWith({phase: "connected"});
    submissionsWsManager.removeListener(secondId);
    expect(socket.closeCalls).toBe(0);
    submissionsWsManager.removeListener(firstId);
    expect(socket.closeCalls).toBe(1);
  });

  it("reuses the socket when the newest listener changes only the replay cursor", async () => {
    const first = listener({url: () => "ws://localhost/ws/submissions?contestId=contest&since=1"});
    add(first);
    await flushMicrotasks();
    const socket = TestWebSocket.instances[0];
    socket.open();
    const second = listener({key: "cursor-2", url: () => "ws://localhost/ws/submissions?contestId=contest&since=2"});
    const id = add(second);
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(second.onResyncRequired).not.toHaveBeenCalled();
    socket.message({event_type: "submissions.queued"});
    expect(second.onMessage).toHaveBeenCalledTimes(1);
    expect(first.onMessage).not.toHaveBeenCalled();
    submissionsWsManager.removeListener(id);
    socket.message({event_type: "submissions.completed"});
    expect(first.onMessage).toHaveBeenCalledTimes(1);
    expect(socket.closeCalls).toBe(0);
  });

  it("resyncs when switching streams and ignores events from the old socket", async () => {
    add(listener());
    await flushMicrotasks();
    const old = TestWebSocket.instances[0];
    old.open();
    const oldMessage = old.onmessage;
    const oldClose = old.onclose;
    const next = listener({key: "other", url: () => "ws://localhost/ws/submissions?contestId=other"});
    add(next);
    await flushMicrotasks();
    expect(old.closeCalls).toBe(1);
    expect(next.onResyncRequired).toHaveBeenCalledTimes(1);
    expect(TestWebSocket.instances).toHaveLength(2);
    TestWebSocket.instances[1].open();
    oldMessage?.({data: "stale"} as MessageEvent);
    oldClose?.({code: 1006, reason: "stale"});
    jest.advanceTimersByTime(5000);
    await flushMicrotasks();
    expect(next.onMessage).not.toHaveBeenCalled();
    expect(next.onStatusChange).toHaveBeenLastCalledWith({phase: "connected"});
    expect(TestWebSocket.instances).toHaveLength(2);
  });

  for (const code of [1000, 1006, 1008, 4408, 4409, 4410]) {
    it(`resyncs and reconnects after close ${code}`, async () => {
      const params = listener();
      add(params);
      await flushMicrotasks();
      TestWebSocket.instances[0].open();
      TestWebSocket.instances[0].disconnect(code);
      expect(params.onStatusChange).toHaveBeenLastCalledWith({phase: "reconnecting", attempt: 1, maxAttempts: 3});
      jest.advanceTimersByTime(1999);
      await flushMicrotasks();
      expect(params.onResyncRequired).toHaveBeenCalledTimes(1);
      jest.advanceTimersByTime(1);
      await flushMicrotasks();
      expect(params.onResyncRequired).toHaveBeenCalledTimes(2);
      expect(TestWebSocket.instances).toHaveLength(2);
      TestWebSocket.instances[1].open();
      expect(params.onFatalConnectionError).toHaveBeenLastCalledWith(false);
      expect(params.onConnectionError).toHaveBeenLastCalledWith(false);
    });
  }

  it("closes a connection that does not open within ten seconds", async () => {
    const params = listener();
    add(params);
    await flushMicrotasks();
    jest.advanceTimersByTime(9999);
    expect(TestWebSocket.instances[0].closeCalls).toBe(0);
    jest.advanceTimersByTime(1);
    await flushMicrotasks();
    expect(TestWebSocket.instances[0].closeCalls).toBe(1);
    jest.advanceTimersByTime(2000);
    await flushMicrotasks();
    expect(params.onResyncRequired).toHaveBeenCalledTimes(2);
    expect(TestWebSocket.instances).toHaveLength(2);
  });

  it("bounds constructor retries without repeating the successful snapshot", async () => {
    TestWebSocket.failConnections = true;
    const params = listener();
    add(params);
    await flushMicrotasks();
    for (const delay of [2000, 4000, 8000]) {
      jest.advanceTimersByTime(delay);
      await flushMicrotasks();
    }
    expect(params.onResyncRequired).toHaveBeenCalledTimes(1);
    expect(params.onConnectionError).toHaveBeenLastCalledWith(true);
    expect(params.onFatalConnectionError).toHaveBeenLastCalledWith(true);
    expect(params.onStatusChange).toHaveBeenLastCalledWith({phase: "fatal", attempt: 3, maxAttempts: 3});
  });

  it("cancels scheduled reconnects when there are no subscribers", async () => {
    const params = listener();
    const id = add(params);
    await flushMicrotasks();
    TestWebSocket.instances[0].open();
    TestWebSocket.instances[0].disconnect();
    submissionsWsManager.removeListener(id);
    jest.advanceTimersByTime(30000);
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(1);
    expect(params.onResyncRequired).toHaveBeenCalledTimes(1);
  });

  it("does not open a socket after an unsubscribed snapshot completes", async () => {
    let finish!: () => void;
    const pending = new Promise<void>(resolve => {
      finish = resolve;
    });
    const id = add(listener({onResyncRequired: () => pending}));
    submissionsWsManager.removeListener(id);
    finish();
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(0);
  });

  it("keeps waiting for a snapshot when another listener joins", async () => {
    let finish!: () => void;
    const pending = new Promise<void>(resolve => {
      finish = resolve;
    });
    add(listener({onResyncRequired: () => pending}));
    add(listener());
    try {
      jest.advanceTimersByTime(5000);
      await flushMicrotasks();
      expect(TestWebSocket.instances).toHaveLength(0);
    } finally {
      finish();
      await flushMicrotasks();
    }
  });
});
