import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import Loading from "@/app/[slug]/(org)/teams/[team_id]/loading";
import Page from "@/app/[slug]/(org)/teams/[team_id]/page";

import {cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

import type {ContestModel, TeamModel} from "@/contracts/core/v1/models";

const params = {slug: "school", team_id: "team"};
const team: TeamModel = {
  id: "team", organization_id: "org", name: "Gate Team", description: "Our team",
  created_at: "2026-10-01", updated_at: "2026-10-01",
};
const contest = (id: string, schedule: Partial<ContestModel> = {}): ContestModel => ({
  id, login: id, organization_id: "org", organization_login: "school", title: id,
  description: "", visibility: "public", monitor_scope: "public", submissions_list_scope: "public",
  submissions_review_scope: "public", submission_details_scope: "public", freeze_status: "auto" as ContestModel["freeze_status"],
  created_by: "user", created_at: "2026-10-01", updated_at: "2026-10-01", ...schedule,
});

describe("Team profile through the production page", () => {
  let cleanupDOM: () => void;
  let restoreFetch: (() => void) | undefined;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(() => {
    cleanup();
    restoreFetch?.();
  });
  afterAll(() => {
    cleanupDOM();
  });

  const mockApi = ({
    teamStatus = 200, membersStatus = 200, contestsStatus = 200, contests = [] as ContestModel[], pending = false,
  } = {}) => {
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), "http://localhost");
      if (url.pathname === "/api/teams/team") {
        return teamStatus === 200 ? Response.json({team}) : Response.json({message: "Team unavailable"}, {status: teamStatus});
      }
      if (url.pathname.endsWith("/me")) {
        return Response.json({user: null});
      }
      if (pending) {
        return new Promise<Response>(() => {});
      }
      if (url.pathname.endsWith("/members")) {
        return membersStatus === 200 ? Response.json({members: [
          {user_id: "alice", team_id: team.id, username: "alice", role: "maintainer", created_at: "2026-10-01"},
          {user_id: "bob", team_id: team.id, username: "bob", role: "member", created_at: "2026-10-01"},
        ], pagination: {page: 1, total: 2}}) : Response.json({message: "Members unavailable"}, {status: membersStatus});
      }
      if (url.pathname.endsWith("/contests")) {
        return contestsStatus === 200 ? Response.json({contests, pagination: {page: 1, total: contests.length}})
          : Response.json({message: "Contests unavailable"}, {status: contestsStatus});
      }
      if (url.pathname.endsWith("/problems")) {
        return Response.json({problems: [], pagination: {page: 1, total: 0}});
      }
      throw new Error(`Unexpected API request: ${url.pathname}`);
    }, {preconnect: () => {}}));
    restoreFetch = () => fetch.mockRestore();
    return fetch;
  };

  const renderPage = async () => {
    const view = renderWithProviders(<Page params={params} />);
    await waitFor(() => expect(view.getByRole("heading", {name: team.name})).toBeTruthy());
    return view;
  };

  it("renders the team badge, supported maintainer role and linked roster handles", async () => {
    const fetch = mockApi();
    const view = await renderPage();
    expect(view.getByRole("heading", {name: team.name})).toBeTruthy();
    expect(view.getByLabelText(`Значок команды ${team.name}`).textContent).toBe("GA");
    expect(view.getByText("Команда")).toBeTruthy();
    expect(view.getByText(team.description!)).toBeTruthy();
    expect(view.getByRole("link", {name: /Назад к организации/}).getAttribute("href")).toBe("/school");
    await waitFor(() => expect(view.getByRole("link", {name: "@alice"}).getAttribute("href")).toBe("/@alice"));
    expect(view.getByRole("link", {name: "@bob"}).getAttribute("href")).toBe("/@bob");
    expect(view.getByText("Организатор (maintainer)")).toBeTruthy();
    expect(view.getByText("Участник (member)")).toBeTruthy();
    expect(view.queryByText(/Капитан/)).toBeNull();
    expect(fetch.mock.calls.some(([url]) => String(url).includes("/teams/team/members"))).toBe(true);

  });

  it("shows active, past, upcoming and unscheduled contests with their destination links", async () => {
    mockApi({contests: [
      contest("Active", {start_time: "2000-01-01", end_time: "2999-01-01"}),
      contest("Past", {start_time: "2000-01-01", end_time: "2001-01-01"}),
      contest("Upcoming", {start_time: "2999-01-01"}),
      contest("Unscheduled"),
    ]});
    const view = await renderPage();
    fireEvent.click(view.getByRole("tab", {name: "Контесты"}));
    await waitFor(() => expect(view.getByRole("link", {name: "Active"})).toBeTruthy());
    for (const name of ["Active", "Past", "Upcoming", "Unscheduled"]) {
      expect(view.getByRole("link", {name}).getAttribute("href")).toBe(`/school/${name}`);
    }
    for (const status of ["Идёт", "Завершён", "Предстоящий", "Без расписания"]) {
      expect(view.getByText(status)).toBeTruthy();
    }
  });

  it("renders the route skeleton and child loading skeletons while requests are pending", async () => {
    const skeleton = renderWithProviders(<Loading />);
    expect(skeleton.getByRole("status", {name: "Загрузка команды"})).toBeTruthy();
    skeleton.unmount();
    mockApi({pending: true});
    const view = await renderPage();
    expect(view.getByRole("status", {name: "Загрузка участников"})).toBeTruthy();
    fireEvent.click(view.getByRole("tab", {name: "Контесты"}));
    expect(view.getByRole("status", {name: "Загрузка контестов команды"})).toBeTruthy();
  });

  it("shows an empty contest state only after a successful empty response", async () => {
    mockApi();
    const view = await renderPage();
    fireEvent.click(view.getByRole("tab", {name: "Контесты"}));
    await waitFor(() => expect(view.getByText("Нет привязанных контестов")).toBeTruthy());
  });

  it("shows a roster authorization error instead of an empty roster", async () => {
    mockApi({membersStatus: 403});
    const view = await renderPage();
    await waitFor(() => expect(view.getByRole("heading", {name: "Доступ запрещён"})).toBeTruthy());
    expect(view.queryByText("Нет участников")).toBeNull();
  });

  it("shows a contest error instead of claiming there is no participation", async () => {
    mockApi({contestsStatus: 500});
    const view = await renderPage();
    fireEvent.click(view.getByRole("tab", {name: "Контесты"}));
    await waitFor(() => expect(view.getByRole("heading", {name: "Ошибка сервера"})).toBeTruthy());
    expect(view.queryByText("Нет привязанных контестов")).toBeNull();
  });

  for (const [status, title] of [[401, "Требуется авторизация"], [403, "Доступ запрещён"], [500, "Ошибка сервера"]] as const) {
    it(`renders the page error for HTTP ${status}`, async () => {
      mockApi({teamStatus: status});
      const view = renderWithProviders(<Page params={params} />);
      await waitFor(() => expect(view.getByRole("heading", {name: title})).toBeTruthy());
      expect(view.queryByRole("tab", {name: "Участники"})).toBeNull();
      if (status === 401) {
        expect(view.getByRole("link", {name: "Войти"}).getAttribute("href")).toBe("/auth/login");
      }

    });
  }

  it("shows not-found for a missing team", async () => {
    mockApi({teamStatus: 404});
    const view = renderWithProviders(<Page params={params} />);
    await waitFor(() => expect(view.getByRole("heading", {name: "Не найдено"})).toBeTruthy());
  });
  for (const slug of ["@alice", "%invalid"]) {
    it(`rejects invalid organization slug ${slug} without fetching a team`, async () => {
      const fetch = mockApi();
      const view = renderWithProviders(<Page params={{slug, team_id: team.id}} />);
      await waitFor(() => expect(view.getByRole("heading", {name: "Не найдено"})).toBeTruthy());
      expect(fetch.mock.calls.some(([url]) => String(url).includes("/teams/"))).toBe(false);
    });
  }
});
