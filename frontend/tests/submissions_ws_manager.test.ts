import {describe, expect, it, jest, mock} from "bun:test";

import {connectSubmissions} from "@/lib/submissionsConnection";

import {flushMicrotasks, TestWebSocket as TestSocket} from "./websocket-test-utils";

describe("Submission connection retry backoff", () => {
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
      const disconnect = connectSubmissions({
        url: () => "ws://localhost/ws/submissions?contestId=contest",
        onMessage: () => {},
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
        } else {
          jest.advanceTimersByTime(8000);
          await flushMicrotasks();
          expect(resync).toHaveBeenCalledTimes(4);
          expect(TestSocket.instances).toHaveLength(1);
          expect(status).toHaveBeenLastCalledWith({phase: "fatal", attempt: 3, maxAttempts: 3});
        }
      } finally {
        disconnect();
        jest.useRealTimers();
        globalThis.WebSocket = originalWebSocket;
      }
    });
  }
});
