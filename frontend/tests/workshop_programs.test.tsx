import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopComponentsTab} from "@/components/workshop/WorkshopComponentsTab";

import {cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

import type {ProblemDraft} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const generator = (name: string, source: string) => ({name, language: "cpp20", source});

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker: null, interactor: null, validators: [],
  generators: [generator("first", "FIRST SOURCE"), generator("second", "SECOND SOURCE")], solutions: [],
  main_solution: null, statements: [], assets: [], subtasks: [], tests: [], library: [],
};

const mockApi = (stored: ProblemDraft): void => {
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), "http://localhost");
      if (url.pathname === "/api/languages") {
        return Promise.resolve(Response.json({languages: [{id: "cpp20", name: "C++20", extensions: ["cpp"], syntax: "cpp"}]}));
      }
      if (init?.method === "PUT") {
        return Promise.resolve(Response.json({...(JSON.parse(String(init.body)) as ProblemDraft), revision: 4}));
      }
      return Promise.resolve(Response.json(stored));
    },
    {preconnect: () => {}},
  ));
  restore = () => fetch.mockRestore();
};

describe("workshop programs", () => {
  it("after deleting a program, the editor shows the program it falls back to", async () => {
    mockApi(draft);
    const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="generator" />);
    await waitFor(() => expect(view.getByDisplayValue("FIRST SOURCE")).toBeTruthy());
    fireEvent.click(view.getByRole("button", {name: "Удалить"}));
    const dialog = await view.findByRole("dialog");
    fireEvent.click(Array.from(dialog.querySelectorAll("button")).find((b) => b.textContent === "Удалить") as HTMLElement);
    await waitFor(() => expect(view.getByDisplayValue("SECOND SOURCE")).toBeTruthy());
    expect(view.queryByDisplayValue("FIRST SOURCE")).toBeNull();
  });
});
