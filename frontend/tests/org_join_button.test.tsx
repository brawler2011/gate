import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import {AppRouterContext} from "next/dist/shared/lib/app-router-context.shared-runtime";
import {SearchParamsContext} from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import React from "react";

import Page from "@/app/[slug]/(org)/page";

import {cleanup, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

import type {OrganizationJoinRequestModel, OrganizationModel, UserModel} from "@/contracts/core/v1";

const router = {
  back: () => {}, forward: () => {}, refresh: () => {},
  push: () => {}, replace: () => {}, prefetch: () => {}, bfcacheId: "test",
};
const org: OrganizationModel = {
  id: "org", login: "school", name: "School", join_policy: "by_request" as OrganizationModel["join_policy"],
  created_at: "2026-10-01", updated_at: "2026-10-01",
};
const user: UserModel = {
  id: "user", username: "alice", role: "user", createdAt: "2026-10-01", updatedAt: "2026-10-01",
};
const pendingRequest: OrganizationJoinRequestModel = {
  id: "request", organization_id: org.id, organization_name: org.name, organization_login: org.login,
  user_id: user.id, username: user.username, status: "pending", created_at: "2026-10-01", updated_at: "2026-10-01",
};

describe("Organization join status through the organization page", () => {
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

  const renderPage = async ({member = false, pending = false, membersError = 403, authenticated = true} = {}) => {
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
      const url = new URL(String(input));
      if (url.pathname.endsWith("/members")) {
        return member ? Response.json({
          members: [{user_id: user.id, organization_id: org.id, username: user.username, role: "member", created_at: "2026-10-01"}],
          pagination: {page: 1, total: 1},
        }) : Response.json({message: "Cannot list members"}, {status: membersError});
      }
      if (url.pathname.endsWith("/requests/mine")) {
        return Response.json({request: pending ? pendingRequest : null});
      }
      if (url.pathname === "/organizations/school") {
        return Response.json({organization: org});
      }
      if (url.pathname.endsWith("/me")) {
        return authenticated ? Response.json({user}) : Response.json({message: "Unauthorized"}, {status: 401});
      }
      if (url.pathname.endsWith("/contests")) {
        return Response.json({contests: [], pagination: {page: 1, total: 1}});
      }
      throw new Error(`Unexpected API request: ${url.pathname}`);
    }, {preconnect: () => {}}));
    restoreFetch = () => fetch.mockRestore();
    const page = await Page({params: Promise.resolve({slug: org.login}), searchParams: Promise.resolve({})});
    const view = renderWithProviders(
      <AppRouterContext.Provider value={router}>
        <SearchParamsContext.Provider value={new URLSearchParams()}>{page}</SearchParamsContext.Provider>
      </AppRouterContext.Provider>
    );
    return {view, fetch};
  };

  it("allows a nonmember without a pending request to apply", async () => {
    const {view, fetch} = await renderPage();
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).includes("requests/mine"))).toBe(true));
    expect(view.getByRole("button", {name: "Подать заявку на вступление"})).toBeTruthy();
    expect(view.queryByText("Участник организации")).toBeNull();
  });

  it("shows pending approval instead of a duplicate join action", async () => {
    const {view} = await renderPage({pending: true});
    await waitFor(() => expect(view.getByText("Заявка на рассмотрении")).toBeTruthy());
    expect(view.queryByRole("button", {name: "Подать заявку на вступление"})).toBeNull();
    expect(view.getByRole("button", {name: "Отменить заявку"})).toBeTruthy();
  });

  it("passes accepted membership through the tab and skips pending request lookup", async () => {
    const {view, fetch} = await renderPage({member: true});
    expect(view.getByText("Участник организации")).toBeTruthy();
    expect(view.queryByRole("button", {name: "Подать заявку на вступление"})).toBeNull();
    expect(fetch.mock.calls.some(([url]) => String(url).includes("requests/mine"))).toBe(false);
  });

  it("shows unexpected membership errors instead of offering to rejoin", async () => {
    const {view} = await renderPage({membersError: 500});
    expect(view.getByText("Ошибка сервера")).toBeTruthy();
    expect(view.queryByRole("button", {name: "Подать заявку на вступление"})).toBeNull();
  });

  it("does not look up membership or offer a join action to guests", async () => {
    const {view, fetch} = await renderPage({authenticated: false});
    expect(view.queryByRole("button", {name: "Подать заявку на вступление"})).toBeNull();
    expect(fetch.mock.calls.some(([url]) => /members|requests\/mine/.test(String(url)))).toBe(false);
  });
});
