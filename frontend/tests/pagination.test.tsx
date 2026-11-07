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

const history = (page: number, pending = false) => ({
  packages: Array.from({length: page < 3 ? 50 : 1}, (_, i) => ({
    id: `package-${101 - (page - 1) * 50 - i}`, version: 101 - (page - 1) * 50 - i,
    status: pending ? "pending" : "ready", created_at: "2026-01-01", package_hash: "hash",
  })),
  pagination: pagination(page),
});

describe("List pagination through production components", () => {
  it("shows three pages for 101 contest participants and requests the last page", async () => {
    mockApi((url) => {
      if (url.pathname.endsWith("/members")) {
        const page = Number(url.searchParams.get("page"));
        return Response.json({members: [{user_id: `user-${page}`, username: `user-${page}`, contest_role: "participant"}],
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

  it("enumerates package history and returns to the newest page after publication", async () => {
    const fetch = mockApi((url, init) => init?.method === "POST"
      ? Response.json({version: 102})
      : Response.json(history(Number(url.searchParams.get("page")))));
    const view = renderWithProviders(<WorkshopPackagesTab problemId="problem" />);
    await waitFor(() => expect(view.getByText("v101")).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "3"}));
    await waitFor(() => expect(view.getByText("v1")).toBeTruthy());
    expect(view.queryByText("v101")).toBeNull();
    fireEvent.click(view.getByRole("button", {name: "Сбилдить пакет"}));
    await waitFor(() => expect(view.getByText("v101")).toBeTruthy());
    expect(fetch.mock.calls.some(([url, init]) => String(url).endsWith("/publish") && init?.method === "POST")).toBe(true);
  });

  it("keeps an old version reachable across pages with no ready packages and retryable errors", async () => {
    let secondAttempts = 0;
    let submitted: string | null = null;
    mockApi((url, init) => {
      if (url.pathname.endsWith("/packages")) {
        const page = Number(url.searchParams.get("page"));
        if (page === 2 && secondAttempts++ === 0) {
          return Response.json({message: "Retry history"}, {status: 500});
        }
        return Response.json(history(page, page < 3));
      }
      if (init?.method === "POST") {
        submitted = url.searchParams.get("package_id");
        return new Response(null, {status: 204});
      }
      return Response.json({});
    });
    const problem = {problem_id: "problem", title: "Example", position: 0, time_limit: 1000, memory_limit: 256} as ContestProblemListItemModel;
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
    fireEvent.click(await view.findByRole("option", {name: /Версия v1 /}));
    fireEvent.click(view.getByRole("button", {name: "Обновить пакет на контесте"}));
    await waitFor(() => expect(submitted).toBe("package-1"));
  });

  it("finds management membership beyond the former twenty-page cap", async () => {
    const fetch = mockApi((url) => {
      if (url.pathname.endsWith("/me")) {
        return Response.json({user: {id: "owner", role: "user"}});
      }
      const page = Number(url.searchParams.get("page"));
      return Response.json({members: page === 21 ? [{user_id: "owner", role: "owner"}]
        : Array.from({length: 100}, (_, index) => ({user_id: `${page}-${index}`, role: "member"})),
      pagination: {page, page_size: 100, total_items: 2001, total_pages: 21}});
    });
    expect(await canManageOrgMembers("org")).toBe(true);
    expect(fetch.mock.calls.some(([url]) => String(url).includes("page=21"))).toBe(true);
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
        user_id: `user-${page}`, username: `user-${page}`, role: "viewer", created_at: "2026-01-01",
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
