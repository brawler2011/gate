import "./setup-dom";

import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {afterEach, expect, it, spyOn} from "bun:test";
import React from "react";

import {SessionProvider, useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import {act, cleanup, fireEvent, render, setupDOMEnvironment, waitFor} from "./test-utils";

import type {UserModel} from "@/contracts/core/v1/schema";

const user: UserModel = {
  id: "user", username: "alice", role: "user", createdAt: "2026-10-01", updatedAt: "2026-10-01",
};
const SessionConsumer = () => {
  const session = useSession();
  return <>
    <span>{session.user?.username ?? "guest"}</span>
    <button onClick={() => session.setUser(null)}>Logout</button>
    <button onClick={() => session.setUser(user)}>Login</button>
  </>;
};
afterEach(cleanup);

it("updates the active session and its query cache on logout and login", async () => {
  const cleanupDOM = setupDOMEnvironment();
  const fetch = spyOn(globalThis, "fetch").mockImplementation((async () => Response.json({user})) as unknown as typeof globalThis.fetch);
  const client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
  try {
    const view = render(<QueryClientProvider client={client}>
      <SessionProvider initialUser={user}><SessionConsumer /></SessionProvider>
    </QueryClientProvider>);
    client.setQueryData(["private-data"], {secret: true});
    expect(view.getByText("alice")).toBeTruthy();
    fireEvent.click(view.getByRole("button", {name: "Logout"}));
    await waitFor(() => expect(view.getByText("guest")).toBeTruthy());
    expect(client.getQueryData(api.queryOptions("get", "/users/me").queryKey)).toEqual({user: null});
    expect(client.getQueryData(["private-data"])).toBeUndefined();
    fireEvent.click(view.getByRole("button", {name: "Login"}));
    await waitFor(() => expect(view.getByText("alice")).toBeTruthy());
    expect(client.getQueryData(api.queryOptions("get", "/users/me").queryKey)).toEqual({user});
  } finally {
    cleanupDOM();
    client.clear();
    fetch.mockRestore();
  }
});

it("does not restore the previous user when an in-flight session fetch finishes after logout", async () => {
  const cleanupDOM = setupDOMEnvironment();
  let finishFetch!: (response: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finishFetch = resolve;
  });
  const fetch = spyOn(globalThis, "fetch").mockImplementation((() => pending) as unknown as typeof globalThis.fetch);
  const client = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
  try {
    const view = render(<QueryClientProvider client={client}>
      <SessionProvider initialUser={user}><SessionConsumer /></SessionProvider>
    </QueryClientProvider>);
    const refetch = client.refetchQueries({queryKey: api.queryOptions("get", "/users/me").queryKey});
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    fireEvent.click(view.getByRole("button", {name: "Logout"}));
    await waitFor(() => expect(view.getByText("guest")).toBeTruthy());
    await act(async () => {
      finishFetch(Response.json({user}));
      await pending;
      await refetch;
    });
    expect(view.getByText("guest")).toBeTruthy();
    expect(client.getQueryData(api.queryOptions("get", "/users/me").queryKey)).toEqual({user: null});
  } finally {
    cleanupDOM();
    client.clear();
    fetch.mockRestore();
  }
});
