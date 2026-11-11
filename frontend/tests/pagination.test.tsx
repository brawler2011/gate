import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {ParticipantsSection} from "@/components/contests/ParticipantsSection";
import {ProblemsSection} from "@/components/contests/ProblemsSection";
import {ProblemMembersManagement} from "@/components/problems/ProblemMembersManagement";
import {WorkshopPackagesTab} from "@/components/workshop/WorkshopPackagesTab";
import {canManageOrgMembers} from "@/lib/permissions";

import {cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

import type {ContestProblemListItemModel} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const pagination = (page: number, items = 101) => ({
  page, page_size: 50, total_items: items, total_pages: Math.ceil(items / 50),
});
const mockApi = (handler: (url: URL, init?: RequestInit) => Response | Promise<Response>) => {
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: RequestInfo | URL, init?: RequestInit) =>
      Promise.resolve(handler(new URL(String(input), "http://localhost"), init)),
    {preconnect: () => {}},
  ));
  restore = () => fetch.mockRestore();
  return fetch;
};

const history = (page: number) => ({
  packages: Array.from({length: page < 3 ? 50 : 1}, (_, i) => {
    const revision = 101 - (page - 1) * 50 - i;
    return {
      id: `package-${revision}`, problem_id: "problem", build_id: `build-${revision}`, revision,
      created_at: "2026-01-01T00:00:00Z", package_hash: `${String(revision).padStart(8, "0")}${"a".repeat(56)}`,
    };
  }),
  pagination: pagination(page),
});

// A draft with every component a build requires, so the build button is enabled.
const buildableDraft = {
  problem_id: "problem", revision: 7, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker_id: "checker", validator_id: null, interactor_id: null,
  main_solution_id: null, files: {}, components: [], assets: [],
  subtasks: [{id: "s", ordinal: 1, name: "All", points: 1, policy: "complete", depends_on: []}],
  tests: [{
    id: "t", ordinal: 1, subtask_id: "s", sample: false, points: 1,
    input: {mode: "manual", object_id: "in", component_id: null, args: [], seed: null},
    answer: {mode: "provided", object_id: "ans", component_id: null},
  }],
};

describe("List pagination through production components", () => {
  it("shows three pages for 101 contest participants and requests the last page", async () => {
    mockApi((url) => {
      if (url.pathname.endsWith("/members")) {
        const page = Number(url.searchParams.get("page"));
        return Response.json({members: [{capabilities: ["change_role", "remove"], user_id: `user-${page}`, username: `user-${page}`, contest_role: "participant"}],
          pagination: pagination(page)});
      }
      return Response.json({requests: [], teams: []});
    });
    const view = renderWithProviders(<ParticipantsSection orgLogin="org" contestLogin="round" />);
    await waitFor(() => expect(view.getByText("@user-1")).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "3"}));
    await waitFor(() => expect(view.getByText("@user-3")).toBeTruthy());
    expect(view.queryByText("@user-1")).toBeNull();
  });

  it("enumerates package history and builds from the draft revision the author last saw", async () => {
    let built: {url: URL; init?: RequestInit} | undefined;
    mockApi((url, init) => {
      if (url.pathname.endsWith("/packages/build")) {
        built = {url, init};
        return Response.json({build_id: "build", revision: 7}, {status: 202});
      }
      if (url.pathname.endsWith("/draft")) {
        return Response.json(buildableDraft);
      }
      if (url.pathname.endsWith("/builds")) {
        return Response.json([]);
      }
      return Response.json(history(Number(url.searchParams.get("page"))));
    });
    const view = renderWithProviders(<WorkshopPackagesTab problemId="problem" />);
    await waitFor(() => expect(view.getByText("00000101")).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "3"}));
    await waitFor(() => expect(view.getByText("00000001")).toBeTruthy());
    expect(view.queryByText("00000101")).toBeNull();
    await waitFor(() => expect((view.getByRole("button", {name: "Собрать пакет"}) as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(view.getByRole("button", {name: "Собрать пакет"}));
    await waitFor(() => expect(built).toBeDefined());
    expect(JSON.parse(String(built?.init?.body))).toEqual({expected_revision: 7});
    expect(new Headers(built?.init?.headers).get("Idempotency-Key")).toBeTruthy();
  });

  it("keeps an old package reachable across pages with retryable errors", async () => {
    let secondAttempts = 0;
    let submitted: string | null = null;
    mockApi((url, init) => {
      if (url.pathname.endsWith("/packages")) {
        const page = Number(url.searchParams.get("page"));
        if (page === 2 && secondAttempts++ === 0) {
          return Response.json({message: "Retry history"}, {status: 500});
        }
        return Response.json(history(page));
      }
      if (init?.method === "POST") {
        submitted = url.searchParams.get("package_id");
        return new Response(null, {status: 204});
      }
      return Response.json({});
    });
    const problem = {capabilities: ["relink_package", "remove", "reorder"], problem_id: "problem", title: "Example", position: 0, time_limit: 1000, memory_limit: 256} as ContestProblemListItemModel;
    const view = renderWithProviders(<ProblemsSection orgLogin="org" contestLogin="round" initialProblems={[problem]} />);
    fireEvent.click(view.getByRole("button", {name: "Заменить пакет Example"}));
    await waitFor(() => expect(view.getByRole("button", {name: "Ещё"})).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "Ещё"}));
    await waitFor(() => expect(view.getByRole("button", {name: "Повторить загрузку"})).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "Повторить загрузку"}));
    await waitFor(() => expect(view.getByRole("button", {name: "Ещё"})).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "Ещё"}));
    const input = await view.findByRole("textbox", {name: "Выберите версию пакета"});
    fireEvent.click(input);
    fireEvent.click(await view.findByRole("option", {name: /Ревизия 1 /}));
    fireEvent.click(view.getByRole("button", {name: "Обновить пакет на контесте"}));
    await waitFor(() => expect(submitted).toBe("package-1"));
  });

  it("uses organization capabilities without fetching member pages", async () => {
    const fetch = mockApi(() => Response.json({organization: {capabilities: ["manage_members"]}}));
    expect(await canManageOrgMembers("org")).toBe(true);
    expect(fetch.mock.calls).toHaveLength(1);
  });

  it("moves back when deletion empties the last member page", async () => {
    let removed = false;
    mockApi((url, init) => {
      if (init?.method === "DELETE") {
        removed = true;
        return new Response(null, {status: 204});
      }
      const page = Number(url.searchParams.get("page"));
      return Response.json({members: removed && page === 3 ? [] : [{
        capabilities: ["change_role", "remove"], user_id: `user-${page}`, username: `user-${page}`, role: "viewer", created_at: "2026-01-01",
      }], pagination: pagination(page, removed ? 100 : 101)});
    });
    const view = renderWithProviders(<ProblemMembersManagement problemId="problem" />);
    await waitFor(() => expect(view.getByText("user-1")).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "3"}));
    await waitFor(() => expect(view.getByText("user-3")).toBeTruthy());
    const row = view.getByText("user-3").closest("tr")!;
    fireEvent.click(row.querySelectorAll("button")[1]);
    await waitFor(() => expect(view.getByText("user-2")).toBeTruthy());
  });
});
