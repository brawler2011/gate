import "./setup-dom";

import {notifications} from "@mantine/notifications";
import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {ContestTeamsManagement} from "@/components/contests/ContestTeamsManagement";
import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";

import {act, cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor, within} from "./test-utils";

describe("Production entity management", () => {
  let cleanupDOM: () => void;
  let restoreFetch: () => void;
  let notification: ReturnType<typeof spyOn>;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
    notification = spyOn(notifications, "show").mockReturnValue("test");
  });
  afterEach(() => {
    cleanup(); restoreFetch?.(); notification.mockClear();
  });
  afterAll(() => {
    notification.mockRestore(); cleanupDOM();
  });

  const mockFetch = (handler: (url: URL, init?: RequestInit) => Promise<Response>) => {
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      (input: RequestInfo | URL, init?: RequestInit) => handler(new URL(String(input).replace(/^\/api/, ""), "http://localhost"), init), {preconnect: () => {}}
    ));
    restoreFetch = () => fetch.mockRestore();
    return fetch;
  };

  it.each([false, true])("keeps contest team deletion behavior with failure=%s", async (failure) => {
    let deleted = false;
    mockFetch(async (_url, init) => {
      if (init?.method === "DELETE") {
        if (failure) {
          return Response.json({message: "Denied"}, {status: 403});
        }
        deleted = true;
        return new Response(null, {status: 204});
      }
      return Response.json({teams: deleted ? [] : [{team_id: "team", team_name: "Team A", contest_role: "participant"}]});
    });
    const view = renderWithProviders(<ContestTeamsManagement orgLogin="org" contestLogin="round" />);
    expect(view.queryByText("Team A")).toBeNull();
    expect((view.getByRole("button", {name: "Добавить"}) as HTMLButtonElement).disabled).toBe(true);
    await waitFor(() => expect(view.getByText("Team A")).toBeTruthy());
    const row = view.getByText("Team A").closest("tr")!;
    await act(async () => {
      fireEvent.click(within(row).getAllByRole("button")[1]);
    });
    if (failure) {
      expect(view.getByText("Team A")).toBeTruthy();
      expect(notification).toHaveBeenCalledWith(expect.objectContaining({message: "Denied", color: "red"}));
    } else {
      await waitFor(() => expect(view.getByText("Нет команд. Добавьте команды для привязки к контесту")).toBeTruthy());
    }
  });

  it.each([false, true])("loads organization tabs and preserves removal behavior with failure=%s", async (failure) => {
    let removed = false;
    mockFetch(async (url, init) => {
      if (init?.method === "DELETE") {
        if (failure) {
          return Response.json({message: "Denied"}, {status: 403});
        }
        removed = true;
        return new Response(null, {status: 204});
      }
      if (url.pathname.endsWith("/invitations")) {
        return Response.json({invitations: []});
      }
      if (url.pathname.endsWith("/requests")) {
        return Response.json({requests: []});
      }
      return Response.json({members: [
        {user_id: "owner", username: "owner", role: "owner", created_at: "2026-01-01"},
        ...removed ? [] : [{user_id: "user", username: "alice", role: "member", created_at: "2026-01-01"}],
      ]});
    });
    const view = renderWithProviders(<OrgMembersManagement orgLogin="org" />);
    await waitFor(() => expect(view.getByText("@alice")).toBeTruthy());
    expect(within(view.getByText("@owner").closest("tr")!).queryByRole("button")).toBeNull();
    await act(async () => {
      fireEvent.click(within(view.getByText("@alice").closest("tr")!).getByRole("button"));
    });
    if (failure) {
      await waitFor(() => expect(notification).toHaveBeenCalledWith(expect.objectContaining({message: "Denied", color: "red"})));
      expect(view.getByText("@alice")).toBeTruthy();
    } else {
      await waitFor(() => expect(view.queryByText("@alice")).toBeNull());
    }
    fireEvent.click(view.getByRole("tab", {name: "Приглашения"}));
    expect(view.getByText("Нет активных приглашений")).toBeTruthy();
    fireEvent.click(view.getByRole("tab", {name: "Заявки"}));
    expect(view.getByText("Нет активных заявок на вступление")).toBeTruthy();
  });

  it.each([false, true])("updates contest roles with failure=%s", async (failure) => {
    let role = "participant";
    mockFetch(async (url, init) => {
      if (init?.method === "PATCH") {
        if (failure) {
          return Response.json({message: "Denied"}, {status: 403});
        }
        role = url.searchParams.get("role") ?? role;
        return new Response(null, {status: 204});
      }
      return Response.json({teams: [{team_id: "team", team_name: "Team A", contest_role: role}]});
    });
    const view = renderWithProviders(<ContestTeamsManagement orgLogin="org" contestLogin="round" />);
    await waitFor(() => expect(view.getByText("Team A")).toBeTruthy());
    fireEvent.click(within(view.getByText("Team A").closest("tr")!).getAllByRole("button")[0]);
    fireEvent.click(view.getByRole("textbox", {name: "Новая роль"}));
    fireEvent.click(view.getByRole("option", {name: "Модератор"}));
    await act(async () => {
      fireEvent.click(view.getByRole("button", {name: "Применить"}));
    });
    if (failure) {
      await waitFor(() => expect(notification).toHaveBeenCalledWith(expect.objectContaining({message: "Denied"})));
    } else {
      await waitFor(() => expect(within(view.getByText("Team A").closest("tr")!).getByText("Модератор")).toBeTruthy());
    }
  });

  it.each(["approve", "reject", "cancel"])("refreshes the correct organization collections after %s", async (action) => {
    let changed = false;
    mockFetch(async (url, init) => {
      if (init?.method !== "GET") {
        changed = true;
        return new Response(null, {status: 204});
      }
      if (url.pathname.endsWith("/invitations")) {
        return Response.json({invitations: changed ? [] : [{id: "invite", username: "invited", role: "member", created_at: "2026-01-01", inviter_username: "owner"}]});
      }
      if (url.pathname.endsWith("/requests")) {
        return Response.json({requests: changed ? [] : [{id: "request", username: "alice", created_at: "2026-01-01"}]});
      }
      return Response.json({members: changed && action === "approve" ? [{user_id: "user", username: "alice", role: "member", created_at: "2026-01-01"}] : []});
    });
    const view = renderWithProviders(<OrgMembersManagement orgLogin="org" />);
    fireEvent.click(view.getByRole("tab", {name: new RegExp(action === "cancel" ? "Приглашения" : "Заявки")}));
    await waitFor(() => expect(view.getByText(action === "cancel" ? "@invited" : "@alice")).toBeTruthy());
    await act(async () => {
      fireEvent.click(view.getByRole("button", {name: {cancel: "Отменить", approve: "Одобрить", reject: "Отклонить"}[action]}));
    });
    if (action === "approve") {
      await act(async () => {
        fireEvent.click(view.getByRole("button", {name: "Подтвердить"}));
      });
    }
    await waitFor(() => expect(notification).toHaveBeenCalled());
    await waitFor(() => expect(view.getByText(action === "cancel" ? "Нет активных приглашений" : "Нет активных заявок на вступление")).toBeTruthy());
    if (action === "approve") {
      fireEvent.click(view.getByRole("tab", {name: /Участники/}));
      expect(view.getByText("@alice")).toBeTruthy();
    }
  });

  it("adds a searched team as participant, clears selection and refreshes the list", async () => {
    let added: {team_id: string; team_name: string; contest_role: string} | undefined;
    mockFetch(async (url, init) => {
      if (init?.method === "POST") {
        const teamId = url.searchParams.get("team_id");
        if (teamId !== "team") {
          return Response.json({message: "Unknown team"}, {status: 404});
        }
        added = {team_id: teamId, team_name: "Team A", contest_role: url.searchParams.get("role") ?? ""};
        return new Response(null, {status: 204});
      }
      if (url.pathname === "/teams") {
        return Response.json({teams: [{id: "team", name: "Team A"}]});
      }
      return Response.json({teams: added ? [added] : []});
    });
    const view = renderWithProviders(<ContestTeamsManagement orgLogin="org" contestLogin="round" orgId="org-id" />);
    await waitFor(() => expect(view.getByText("Нет команд. Добавьте команды для привязки к контесту")).toBeTruthy());
    const input = view.getByPlaceholderText("Поиск команды по названию...");
    fireEvent.focus(input);
    fireEvent.change(input, {target: {value: "Team"}});
    fireEvent.keyUp(input, {key: "m", code: "KeyM"});
    await waitFor(() => expect(view.getByRole("option", {name: "Team A"})).toBeTruthy());
    fireEvent.click(view.getByRole("option", {name: "Team A"}));
    await act(async () => {
      fireEvent.click(view.getByRole("button", {name: "Добавить"}));
    });
    await waitFor(() => expect(view.getByText("Team A")).toBeTruthy());
    expect(within(view.getByText("Team A").closest("tr")!).getByText("Участник")).toBeTruthy();
    expect((input as HTMLInputElement).value).toBe("");
    expect((view.getByRole("button", {name: "Добавить"}) as HTMLButtonElement).disabled).toBe(true);
  });

  it.each([false, true])("invites searched users with failure=%s", async (failure) => {
    let invited: {id: string; username: string; role: string; created_at: string; inviter_username: string} | undefined;
    mockFetch(async (url, init) => {
      if (init?.method === "POST") {
        if (failure) {
          return Response.json({message: "Denied"}, {status: 403});
        }
        const body = JSON.parse(String(init.body)) as {user_id: string; role: string};
        if (body.user_id !== "user") {
          return Response.json({message: "Unknown user"}, {status: 404});
        }
        invited = {id: "invite", username: "alice", role: body.role, created_at: "2026-01-01", inviter_username: "owner"};
        return new Response(null, {status: 204});
      }
      if (url.pathname === "/users") {
        return Response.json({users: [{id: "user", username: "alice"}]});
      }
      if (url.pathname.endsWith("/members")) {
        return Response.json({members: []});
      }
      if (url.pathname.endsWith("/requests")) {
        return Response.json({requests: []});
      }
      return Response.json({invitations: invited ? [invited] : []});
    });
    const view = renderWithProviders(<OrgMembersManagement orgLogin="org" />);
    fireEvent.click(view.getByRole("button", {name: "Пригласить"}));
    const input = await view.findByPlaceholderText("Введите имя пользователя...");
    fireEvent.focus(input);
    fireEvent.change(input, {target: {value: "alice"}});
    fireEvent.keyUp(input, {key: "e", code: "KeyE"});
    await waitFor(() => expect(view.getByRole("option", {name: "alice"})).toBeTruthy());
    fireEvent.click(view.getByRole("option", {name: "alice"}));
    await act(async () => {
      fireEvent.click(view.getByRole("button", {name: "Отправить приглашение"}));
    });
    await waitFor(() => expect(notification).toHaveBeenCalled());
    if (failure) {
      await waitFor(() => expect(notification).toHaveBeenCalledWith(expect.objectContaining({message: "Denied", color: "red"})));
      expect(view.getByRole("dialog")).toBeTruthy();
    } else {
      fireEvent.click(view.getByRole("tab", {name: /Приглашения/}));
      await waitFor(() => expect(view.getByText("@alice")).toBeTruthy());
      expect(within(view.getByText("@alice").closest("tr")!).getByText("Участник")).toBeTruthy();
    }
  });

});
