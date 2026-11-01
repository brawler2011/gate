import "./setup-dom";

import {notifications} from "@mantine/notifications";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {afterEach, beforeEach, expect, it, spyOn} from "bun:test";
import React from "react";

import {ContestEventsListener} from "@/components/contests/ContestEventsListener";
import {ContestMonitorTable} from "@/components/contests/ContestMonitorTable";
import {ContestMessagesClient} from "@/components/contests/messages/ContestMessagesClient";
import {SubmissionsProvider} from "@/contexts/SubmissionsProvider";
import {env} from "@/lib/env";

import {Monitor} from "./monitor-test-utils";
import {act, cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";
import {flushMicrotasks, TestWebSocket} from "./websocket-test-utils";

import type {ContestModel, UserModel} from "@/contracts/core/v1/schema";

const pagination = {page: 1, total: 0};
const originalSocket = globalThis.WebSocket;
let fetch: ReturnType<typeof spyOn<typeof globalThis, "fetch">>;
let show: ReturnType<typeof spyOn<typeof notifications, "show">>;
let wsUrl: ReturnType<typeof spyOn<typeof env, "getWebSocketUrl">>;
beforeEach(() => {
  TestWebSocket.instances = [];
  TestWebSocket.failConnections = false;
  globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
  show = spyOn(notifications, "show").mockReturnValue("notice");
  wsUrl = spyOn(env, "getWebSocketUrl").mockReturnValue("ws://localhost/ws");
  fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes("/announcements")) {
      return Response.json({announcements: [], pagination});
    }
    if (url.includes("/clarifications")) {
      return Response.json({clarifications: [], pagination});
    }
    return Response.json({items: [], problems: [], is_frozen: false});
  }, {preconnect: () => {}}));
});
afterEach(async () => {
  cleanup();
  await flushMicrotasks();
  fetch.mockRestore(); show.mockRestore(); wsUrl.mockRestore();
  globalThis.WebSocket = originalSocket;
});

it("uses one contest socket and one notification while refreshing only the affected messages", async () => {
  renderWithProviders(<>
    <ContestEventsListener contestId="contest" currentUserId="user" orgLogin="org" contestLogin="round" />
    <ContestMessagesClient contest={{id: "contest"} as ContestModel} orgLogin="org" contestLogin="round"
      user={{id: "user"} as UserModel} isModerator={false}
      initialAnnouncements={{announcements: [], pagination}} initialClarifications={{clarifications: [], pagination}} />
  </>);
  await act(flushMicrotasks);
  expect(TestWebSocket.instances).toHaveLength(1);
  expect(fetch).not.toHaveBeenCalled();
  const socket = TestWebSocket.instances[0];
  await act(async () => {
    socket.open();
    socket.message({event_type: "contest.announcement_created", payload: {id: "ann", title: "News"}});
  });
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  expect(String(fetch.mock.calls[0][0])).toContain("/announcements");
  expect(show).toHaveBeenCalledTimes(1);
  await act(async () => socket.message({event_type: "contest.clarification_answered", payload: {user_id: "user", answer: "Yes"}}));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(String(fetch.mock.calls[1][0])).toContain("/clarifications");
  await act(async () => socket.message({event_type: "contest.announcement_deleted", payload: {id: "ann"}}));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  expect(String(fetch.mock.calls[2][0])).toContain("/announcements");
  expect(show).toHaveBeenCalledTimes(2);
});

it("loads one scoreboard snapshot per socket connection", async () => {
  renderWithProviders(<Monitor orgLogin="org" contestLogin="round"
    initialScoreboard={{contest_id: "contest", contest_login: "round", organization_login: "org", penalty_per_attempt: 20,
      items: [], problems: [], is_frozen: false}} />);
  await act(flushMicrotasks);
  expect(fetch).not.toHaveBeenCalled();
  expect(TestWebSocket.instances).toHaveLength(1);
  await act(async () => TestWebSocket.instances[0].open());
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
});

it("refreshes the active clarification filter without fetching announcements", async () => {
  const view = renderWithProviders(<>
    <ContestEventsListener contestId="contest" currentUserId="user" orgLogin="org" contestLogin="round" isModerator />
    <ContestMessagesClient contest={{id: "contest"} as ContestModel} orgLogin="org" contestLogin="round"
      user={{id: "user"} as UserModel} isModerator
      initialAnnouncements={{announcements: [], pagination}} initialClarifications={{clarifications: [], pagination}} />
  </>);
  fireEvent.click(view.getByRole("tab", {name: "Вопросы участников"}));
  fireEvent.click(view.getByPlaceholderText("Статус"));
  fireEvent.click(view.getByRole("option", {name: "Ожидают ответа"}));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  expect(String(fetch.mock.calls[0][0])).toContain("status=pending");
  await act(async () => TestWebSocket.instances[0].message({event_type: "contest.clarification_created",
    payload: {user_id: "other", question: "Why?"}}));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(String(fetch.mock.calls[1][0])).toContain("status=pending");
  expect(fetch.mock.calls.every(([url]) => String(url).includes("/clarifications"))).toBe(true);
});

it("keeps the monitor socket while loading each selected snapshot once", async () => {
  fetch.mockImplementation(Object.assign(async () => Response.json({items: [], problems: [], is_frozen: true}),
    {preconnect: () => {}}));
  const view = renderWithProviders(<Monitor orgLogin="org" contestLogin="round" isManager
    initialScoreboard={{contest_id: "contest", contest_login: "round", organization_login: "org", penalty_per_attempt: 20,
      items: [], problems: [], is_frozen: true}} />);
  await act(flushMicrotasks);
  const socket = TestWebSocket.instances[0];
  await act(async () => socket.open());
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  fireEvent.click(view.getByRole("switch"));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  expect(String(fetch.mock.calls[1][0])).toContain("unfrozen=true");
  await waitFor(() => expect((view.getByRole("switch") as HTMLInputElement).disabled).toBe(false));
  fireEvent.click(view.getByRole("switch"));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(3));
  expect(String(fetch.mock.calls[2][0])).toContain("unfrozen=false");
  expect(TestWebSocket.instances).toHaveLength(1);
  expect(socket.closeCalls).toBe(0);
});

it("updates a cached monitor while its table is unmounted without HTTP", async () => {
  const client = new QueryClient({defaultOptions: {queries: {retry: false, staleTime: Infinity, gcTime: Infinity}}});
  let username = "Original participant";
  fetch.mockImplementation(Object.assign(async () => Response.json({is_frozen: false, problems: [], items: [{
    user_id: "user", username, problems_solved: 0, total_penalty: 0, problem_results: [],
  }]}), {preconnect: () => {}}));
  const Screen = () => {
    const [visible, setVisible] = React.useState(true);
    return <QueryClientProvider client={client}>
      <SubmissionsProvider contestId="contest" orgLogin="org" contestLogin="round" userId="user">
        <button onClick={() => setVisible(!visible)}>Toggle monitor</button>
        {visible && <ContestMonitorTable orgLogin="org" contestLogin="round" />}
      </SubmissionsProvider>
    </QueryClientProvider>;
  };
  const view = renderWithProviders(<Screen />);
  try {
    await waitFor(() => expect(view.getByText(username)).toBeTruthy());
    await act(async () => TestWebSocket.instances[0].open());
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    fireEvent.click(view.getByRole('button', {name: 'Toggle monitor'}));
    username = "New participant";
    await act(async () => TestWebSocket.instances[0].message({event_type: "scoreboard.updated", sequence: 8,
      payload: {user_id: "user", unfrozen: false, replace: false, scoreboard: {contest_id: "contest",
        is_frozen: false, problems: [], items: [{user_id: "user", username, problems_solved: 0,
          total_penalty: 0, problem_results: []}]}}}));
    expect(fetch).toHaveBeenCalledTimes(2);
    fireEvent.click(view.getByRole('button', {name: 'Toggle monitor'}));
    await waitFor(() => expect(view.getByText(username)).toBeTruthy());
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(TestWebSocket.instances).toHaveLength(1);
  } finally {
    view.unmount(); client.clear();
  }
});
