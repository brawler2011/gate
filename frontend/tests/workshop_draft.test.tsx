import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {useProblemDraft, useSaveDraft} from "@/components/workshop/draft/useDraft";

import {cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const draft = (revision: number, title: string) => ({
  problem_id: "problem", revision, title, short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker_id: null, validator_id: null, interactor_id: null,
  main_solution_id: null, files: {}, components: [], subtasks: [], tests: [], assets: [],
});

type Call = {method: string; url: URL; init?: RequestInit};

const mockApi = (handler: (call: Call) => Response) => {
  const calls: Call[] = [];
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: RequestInfo | URL, init?: RequestInit) => {
      const call = {method: init?.method ?? "GET", url: new URL(String(input), "http://localhost"), init};
      calls.push(call);
      return Promise.resolve(handler(call));
    },
    {preconnect: () => {}},
  ));
  restore = () => fetch.mockRestore();
  return calls;
};

// A minimal consumer of the production hooks: shows the cached draft and saves a new title on click.
const Editor = () => {
  const {data} = useProblemDraft("problem");
  const {save} = useSaveDraft("problem");
  return (
    <div>
      <span data-testid="title">{data?.title ?? "loading"}</span>
      <button onClick={() => void save((draft) => ({...draft, title: "Mine"}))}>save</button>
    </div>
  );
};

describe("workshop draft editing", () => {
  it("saves with the revision the author read and an idempotency key, then shows the stored draft", async () => {
    const calls = mockApi(({method}) => (method === "PUT" ? Response.json(draft(4, "Mine")) : Response.json(draft(3, "Original"))));
    const view = renderWithProviders(<Editor />);
    await waitFor(() => expect(view.getByTestId("title").textContent).toBe("Original"));
    fireEvent.click(view.getByRole("button", {name: "save"}));
    await waitFor(() => expect(view.getByTestId("title").textContent).toBe("Mine"));
    const put = calls.find((c) => c.method === "PUT") as Call;
    expect(put.url.pathname).toBe("/api/problems/problem/draft");
    expect(put.url.searchParams.get("expected_revision")).toBe("3");
    expect(new Headers(put.init?.headers).get("Idempotency-Key")).toBeTruthy();
    expect(JSON.parse(String(put.init?.body)).title).toBe("Mine");
  });

  it("does not retry a conflicting save and reloads the newer draft", async () => {
    let reads = 0;
    const calls = mockApi(({method}) => {
      if (method === "PUT") {
        return Response.json({message: "conflict"}, {status: 409});
      }
      reads++;
      return Response.json(reads === 1 ? draft(3, "Original") : draft(5, "Changed elsewhere"));
    });
    const view = renderWithProviders(<Editor />);
    await waitFor(() => expect(view.getByTestId("title").textContent).toBe("Original"));
    fireEvent.click(view.getByRole("button", {name: "save"}));
    await waitFor(() => expect(view.getByTestId("title").textContent).toBe("Changed elsewhere"));
    expect(calls.filter((c) => c.method === "PUT")).toHaveLength(1);
  });
});
