import {afterEach, beforeEach, describe, expect, it, jest, mock} from "bun:test";

import {connectSubmissions} from "@/lib/submissionsConnection";

import {flushMicrotasks, TestWebSocket} from "./websocket-test-utils";

type Listener = Required<Parameters<typeof connectSubmissions>[0]>;
const listener = (changes: Partial<Listener> = {}): Listener => ({
  url: () => "ws://localhost/ws/submissions?contestId=contest",
  onMessage: mock(() => {}),
  onStatusChange: mock(() => {}), onResyncRequired: mock(async () => {}), ...changes,
});

describe("WebSocket subscription lifecycle", () => {
  const originalSocket = globalThis.WebSocket;
  let disconnects: (() => void)[];
  const add = (params: Listener) => {
    const disconnect = connectSubmissions(params);
    disconnects.push(disconnect);
    return disconnect;
  };

  beforeEach(() => {
    disconnects = [];
    TestWebSocket.instances = [];
    TestWebSocket.failConnections = false;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    jest.useFakeTimers();
  });
  afterEach(async () => {
    disconnects.forEach(disconnect => disconnect());
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

  it("keeps connections independent and disposes only the owning subscription", async () => {
    const first = listener();
    const second = listener({url: () => "ws://localhost/ws/submissions?contestId=other"});
    const disconnect = add(first);
    add(second);
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(2);
    const [a, b] = TestWebSocket.instances;
    a.open(); b.open();
    a.message({event_type: "submissions.queued"});
    expect(first.onMessage).toHaveBeenCalledTimes(1);
    expect(second.onMessage).not.toHaveBeenCalled();
    const stale = a.onmessage;
    disconnect();
    stale?.({data: "old"} as MessageEvent);
    b.message({event_type: "submissions.completed"});
    expect(first.onMessage).toHaveBeenCalledTimes(1);
    expect(second.onMessage).toHaveBeenCalledTimes(1);
    expect(b.closeCalls).toBe(0);
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
    expect(params.onStatusChange).toHaveBeenLastCalledWith({phase: "fatal", attempt: 3, maxAttempts: 3});
  });

  it("cancels scheduled reconnects when there are no subscribers", async () => {
    const params = listener();
    const disconnect = add(params);
    await flushMicrotasks();
    TestWebSocket.instances[0].open();
    TestWebSocket.instances[0].disconnect();
    disconnect();
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
    const disconnect = add(listener({onResyncRequired: () => pending}));
    disconnect();
    finish();
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(0);
  });

  it("retries an initial snapshot failure and opens with the recovered cursor", async () => {
    let cursor = 1;
    let calls = 0;
    const params = listener({
      url: () => `ws://localhost/ws/submissions?since=${cursor}`,
      onResyncRequired: async () => {
        if (++calls === 1) {
          throw new Error("unavailable");
        }
        cursor = 7;
      },
    });
    add(params);
    await flushMicrotasks();
    expect(TestWebSocket.instances).toHaveLength(0);
    expect(params.onStatusChange).toHaveBeenLastCalledWith({phase: "reconnecting", attempt: 1, maxAttempts: 3});
    jest.advanceTimersByTime(2000);
    await flushMicrotasks();
    expect(TestWebSocket.instances[0].url).toContain("since=7");
  });
});
