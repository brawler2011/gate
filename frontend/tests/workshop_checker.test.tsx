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

const WCMP = "#include \"testlib.h\"\n// wcmp\n";

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker: {language: "cpp20", source: "", standard: "wcmp"}, interactor: null,
  validators: [], generators: [], solutions: [], main_solution_id: null, statements: [], assets: [], subtasks: [],
  tests: [], library: [],
};

const mockApi = (stored: ProblemDraft): ProblemDraft[] => {
  const puts: ProblemDraft[] = [];
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), "http://localhost");
      if (url.pathname === "/api/lib") {
        const testlib = {name: "testlib.h", extensions: ["cpp"], version: "0.9.51", commit: "c", content: "// testlib\n"};
        return Promise.resolve(Response.json({files: [testlib], checkers: [{name: "wcmp", description: "Слова", content: WCMP}]}));
      }
      if (url.pathname === "/api/languages") {
        return Promise.resolve(Response.json({languages: [{id: "cpp20", name: "C++20", extensions: ["cpp"], syntax: "cpp"}]}));
      }
      if (init?.method === "PUT") {
        const saved = JSON.parse(String(init.body)) as ProblemDraft;
        puts.push(saved);
        return Promise.resolve(Response.json({...saved, revision: 4}));
      }
      return Promise.resolve(Response.json(stored));
    },
    {preconnect: () => {}},
  ));
  restore = () => fetch.mockRestore();
  return puts;
};

describe("workshop checker", () => {
  it("making a copy of a standard checker saves its source and the testlib.h it builds with", async () => {
    const puts = mockApi(draft);
    const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="checker" />);
    const button = await view.findByRole("button", {name: "Сделать редактируемую копию"});
    await waitFor(() => expect(button.hasAttribute("disabled")).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0].checker).toEqual({language: "cpp20", source: WCMP, standard: ""});
    // The draft had no testlib.h: the standard checker brought the system one.
    expect(puts[0].library).toEqual([{name: "testlib.h", content: "", auto_update: true}]);
    // The copy is an ordinary checker the author edits.
    await waitFor(() => expect(view.getByRole("button", {name: "Сохранить"})).toBeTruthy());
  });
});
