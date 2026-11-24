import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopComponentsTab} from "@/components/workshop/WorkshopComponentsTab";

import {cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

import type {ProblemBuild, ProblemDraft} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const WCMP = "#include \"testlib.h\"\n// wcmp\n";

const checkerTest = (id: string, verdict: "OK" | "WA") =>
  ({id, input: `${id}-in`, output: `${id}-out`, answer: `${id}-ans`, verdict, points: null});
const checkerTests = [checkerTest("t1", "OK"), checkerTest("t2", "WA")];

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, input_file: "", output_file: "", processes: 0, max_score: null, checker: {language: "cpp20", source: "", protocol: "testlib", args: [], standard: "wcmp", tests: checkerTests},
  interactor: null,
  validators: [], generators: [], solutions: [], main_solution: null, statements: [], assets: [], testset: {id: "root", name: "", aggregate: "sum", stop_on_failure: false, requires: [], feedback: "complete", checker_args: [], validator_args: [], groups: []},
  tests: [], library: [], graders: [], attachments: [],
};

const mockApi = (stored: ProblemDraft, builds: ProblemBuild[] = []): ProblemDraft[] => {
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
      if (url.pathname === "/api/problems/problem/builds") {
        return Promise.resolve(Response.json(builds));
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
    expect(puts[0].checker).toEqual({language: "cpp20", source: WCMP, protocol: "testlib", args: [], standard: "", tests: checkerTests});
    // Customizing a standard checker supplies its missing system includes.
    expect(puts[0].library).toEqual([{name: "testlib.h", content: "", auto_update: true}]);
    // The copy is an ordinary checker the author edits.
    await waitFor(() => expect(view.getByRole("button", {name: "Сохранить"})).toBeTruthy());
  });

  it("shows each checker test with the result of the newest build that ran it", async () => {
    // The newest build failed before running checker tests, so the one before it holds the results.
    const build = (revision: number, programTests: ProblemBuild["program_tests"]): ProblemBuild => ({
      id: `build-${revision}`, problem_id: "problem", organization_id: "org", revision, state: "failed", attempt: 1,
      max_attempts: 3, deadline: "2026-11-20T00:00:00Z", created_at: "2026-11-20T00:00:00Z",
      finished_at: "2026-11-20T00:00:00Z", diagnostic: null, solutions: null, program_tests: programTests,
    });
    mockApi(draft, [
      build(3, null),
      build(2, [
        {id: "t1", verdict: "OK", points: null, mismatch: ""},
        {id: "t2", verdict: "PE", points: null, mismatch: "PE, expected WA"},
      ]),
    ]);
    const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="checker" />);
    await waitFor(() => expect(view.getByText(/Результаты — сборка ревизии 2/)).toBeTruthy());
    const rows = Array.from(view.container.querySelectorAll("tbody tr"));
    expect(rows.map((row) => row.querySelector(".mantine-Badge-label")?.textContent)).toEqual(["OK", "PE"]);
    expect(rows.map((row) => row.querySelector("input")?.value)).toEqual(["OK — принято", "WA — неправильный ответ"]);
    expect(rows[1].querySelector("a")?.getAttribute("href")).toBe("/api/problems/problem/draft/objects/t2-in");
  });
});
