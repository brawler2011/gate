import {describe, expect, it, jest, mock} from "bun:test";

import {submissionsWsManager} from "@/lib/submissionsWsManager";

class TestSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: TestSocket[] = [];
  readyState = TestSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onclose: ((event: {code: number; reason: string}) => void) | null = null;
  constructor(_url: string) {
    TestSocket.instances.push(this);
  }
  open() {
    this.readyState = TestSocket.OPEN;
    this.onopen?.();
  }
  close() {
    this.readyState = TestSocket.CLOSED;
  }
  disconnect() {
    this.close();
    this.onclose?.({code: 1006, reason: "offline"});
  }
}

const flushMicrotasks = async () => {
  for (let i = 0; i < 10; i++) {
    await Promise.resolve();
  }
};

describe("Production WebSocket manager reconnect backoff", () => {
  for (const recover of [true, false]) {
    it(recover ? "retries failed snapshot and reconnects" : "reports fatal after bounded snapshot retries", async () => {
      const originalWebSocket = globalThis.WebSocket;
      globalThis.WebSocket = TestSocket as unknown as typeof WebSocket;
      TestSocket.instances = [];
      jest.useFakeTimers();
      let calls = 0;
      const resync = mock(async () => {
        calls++;
        if (calls > 1 && (!recover || calls === 2)) {
          throw new Error("network unavailable");
        }
      });
      const status = mock(() => {});
      const fatal = mock(() => {});
      const listenerId = submissionsWsManager.addListener({
        key: `retry-${recover}`, url: "ws://localhost/ws/submissions?contestId=contest", enabled: true,
        onMessage: () => {}, onConnectionError: () => {}, onFatalConnectionError: fatal,
        onStatusChange: status, onResyncRequired: resync,
      });
      try {
        await flushMicrotasks();
        expect(TestSocket.instances).toHaveLength(1);
        TestSocket.instances[0].open();
        TestSocket.instances[0].disconnect();
        jest.advanceTimersByTime(2000);
        await flushMicrotasks();
        expect(resync).toHaveBeenCalledTimes(2);
        expect(TestSocket.instances).toHaveLength(1);
        jest.advanceTimersByTime(4000);
        await flushMicrotasks();
        if (recover) {
          expect(resync).toHaveBeenCalledTimes(3);
          expect(TestSocket.instances).toHaveLength(2);
          TestSocket.instances[1].open();
          expect(status).toHaveBeenLastCalledWith({phase: "connected"});
          expect(fatal).toHaveBeenLastCalledWith(false);
        } else {
          jest.advanceTimersByTime(8000);
          await flushMicrotasks();
          expect(resync).toHaveBeenCalledTimes(4);
          expect(TestSocket.instances).toHaveLength(1);
          expect(fatal).toHaveBeenLastCalledWith(true);
          expect(status).toHaveBeenLastCalledWith({phase: "fatal", attempt: 3, maxAttempts: 3});
        }
      } finally {
        submissionsWsManager.removeListener(listenerId);
        jest.useRealTimers();
        globalThis.WebSocket = originalWebSocket;
      }
    });
  }
});
