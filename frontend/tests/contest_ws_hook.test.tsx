import "./setup-dom";

import {notifications} from "@mantine/notifications";
import {afterEach, beforeEach, describe, expect, it, jest, mock, spyOn} from "bun:test";
import {StrictMode} from "react";

import {useContestEventsWebSocket} from "@/hooks/useContestEventsWebSocket";
import {env} from "@/lib/env";

import {act, cleanup, renderHook} from "./test-utils";
import {flushMicrotasks, TestWebSocket} from "./websocket-test-utils";

describe("Contest WebSocket hook", () => {
  const originalSocket = globalThis.WebSocket;
  let show: ReturnType<typeof spyOn<typeof notifications, "show">>;
  let wsUrl: ReturnType<typeof spyOn<typeof env, "getWebSocketUrl">>;

  beforeEach(() => {
    jest.useFakeTimers();
    TestWebSocket.instances = [];
    TestWebSocket.failConnections = false;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    show = spyOn(notifications, "show").mockReturnValue("notice");
    wsUrl = spyOn(env, "getWebSocketUrl").mockReturnValue("ws://localhost/ws");
  });
  afterEach(async () => {
    cleanup();
    await flushMicrotasks();
    show.mockRestore();
    wsUrl.mockRestore();
    jest.useRealTimers();
    globalThis.WebSocket = originalSocket;
  });

  it("connects only for an enabled contest and cleans up on disable", async () => {
    const {rerender} = renderHook(useContestEventsWebSocket, {
      initialProps: {contestId: "contest", enabled: false},
    });
    expect(TestWebSocket.instances).toHaveLength(0);
    rerender({contestId: "contest", enabled: true});
    expect(TestWebSocket.instances[0].url).toBe("ws://localhost/ws/contests?contestId=contest");
    rerender({contestId: "contest", enabled: false});
    expect(TestWebSocket.instances[0].closeCalls).toBe(1);
    await act(async () => {
      jest.advanceTimersByTime(30000);
      await flushMicrotasks();
    });
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it("does not connect without a contest", () => {
    renderHook(() => useContestEventsWebSocket({enabled: true}));
    expect(TestWebSocket.instances).toHaveLength(0);
  });

  it("uses current callbacks and permissions without reopening the socket", () => {
    const first = mock(() => {});
    const second = mock(() => {});
    const {rerender} = renderHook(useContestEventsWebSocket, {
      initialProps: {contestId: "contest", currentUserId: "alice", isModerator: false, onEvent: first},
    });
    const socket = TestWebSocket.instances[0];
    const question = {event_type: "contest.clarification_created", payload: {
      id: "question", user_id: "bob", username: "Bob", question: "Why?", problem_letter: "A",
    }};
    socket.message(question);
    expect(first).toHaveBeenCalledTimes(1);
    expect(show).not.toHaveBeenCalled();
    rerender({contestId: "contest", currentUserId: "alice", isModerator: true, onEvent: second});
    socket.message(question);
    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledWith(question);
    expect(show).toHaveBeenLastCalledWith(expect.objectContaining({title: "Новый вопрос от Bob (Задача A)"}));
    rerender({contestId: "contest", currentUserId: "bob", isModerator: true, onEvent: second});
    socket.message(question);
    expect(show).toHaveBeenCalledTimes(1);
    expect(TestWebSocket.instances).toHaveLength(1);
  });

  it("shows announcements and answers only to the answer recipient", () => {
    renderHook(() => useContestEventsWebSocket({contestId: "contest", currentUserId: "alice"}));
    const socket = TestWebSocket.instances[0];
    socket.message({event_type: "contest.announcement_created", payload: {title: "News", problem_letter: "B"}});
    expect(show).toHaveBeenLastCalledWith(expect.objectContaining({
      title: "Объявление жюри [Задача B]", message: "News", color: "blue",
    }));
    socket.message({event_type: "contest.clarification_answered", payload: {user_id: "bob", answer: "No"}});
    expect(show).toHaveBeenCalledTimes(1);
    socket.message({event_type: "contest.clarification_answered", payload: {user_id: "alice", answer: "Yes"}});
    expect(show).toHaveBeenLastCalledWith(expect.objectContaining({message: "Yes", color: "teal"}));
    socket.onmessage?.({data: "not JSON"} as MessageEvent);
    expect(show).toHaveBeenCalledTimes(2);
  });

  it("backs off to ten seconds and resets the delay after opening", async () => {
    renderHook(() => useContestEventsWebSocket({contestId: "contest"}));
    for (const delay of [1000, 1500, 2250, 3375, 5062.5, 7593.75, 10000, 10000]) {
      const count = TestWebSocket.instances.length;
      TestWebSocket.instances[count - 1].disconnect();
      await act(async () => {
        jest.advanceTimersByTime(Math.floor(delay) - 1);
        await flushMicrotasks();
      });
      expect(TestWebSocket.instances).toHaveLength(count);
      await act(async () => {
        jest.advanceTimersByTime(2);
        await flushMicrotasks();
      });
      expect(TestWebSocket.instances).toHaveLength(count + 1);
    }
    TestWebSocket.instances.at(-1)!.open();
    TestWebSocket.instances.at(-1)!.disconnect();
    await act(async () => {
      jest.advanceTimersByTime(1000);
      await flushMicrotasks();
    });
    expect(TestWebSocket.instances).toHaveLength(10);
  });

  it("cancels the old contest retry and ignores stale callbacks", async () => {
    const {rerender, unmount} = renderHook(useContestEventsWebSocket, {initialProps: {contestId: "first"}});
    const old = TestWebSocket.instances[0];
    const staleMessage = old.onmessage;
    const staleClose = old.onclose;
    old.disconnect();
    rerender({contestId: "second"});
    expect(TestWebSocket.instances[1].url).toContain("contestId=second");
    const staleEvent = {data: JSON.stringify({event_type: "contest.announcement_created", payload: {title: "Old"}})};
    staleMessage?.(staleEvent as MessageEvent);
    staleClose?.({code: 1006, reason: "old"});
    unmount();
    await act(async () => {
      jest.advanceTimersByTime(30000);
      await flushMicrotasks();
    });
    expect(show).not.toHaveBeenCalled();
    expect(TestWebSocket.instances).toHaveLength(2);
  });

  it("leaves one live subscription after StrictMode effect replay", async () => {
    const view = renderHook(() => useContestEventsWebSocket({contestId: "contest"}), {
      wrapper: StrictMode,
    });
    await act(flushMicrotasks);
    expect(TestWebSocket.instances.filter(socket => socket.readyState !== TestWebSocket.CLOSED)).toHaveLength(1);
    view.unmount();
    await act(flushMicrotasks);
    expect(TestWebSocket.instances.every(socket => socket.readyState === TestWebSocket.CLOSED)).toBe(true);
  });
});
