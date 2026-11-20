import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopComponentsTab} from "@/components/workshop/WorkshopComponentsTab";

import {cleanup, renderWithProviders, waitFor} from "./test-utils";

import type {ProblemBuild, ProblemDraft} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const solution = (name: string, verdicts: ProblemDraft["solutions"][number]["verdicts"]) => ({name, language: "cpp20", source: name, verdicts, points: null});

const draft: ProblemDraft = {
  problem_id: "problem", revision: 5, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker: null, interactor: null, validators: [], generators: [],
  solutions: [solution("wrong", ["WA"])], main_solution: null, statements: [], assets: [], subtasks: [], tests: [], library: [],
};

const build = (revision: number, solutions: ProblemBuild["solutions"]): ProblemBuild => ({
  id: `build-${revision}`, problem_id: "problem", organization_id: "org", revision, state: "failed", attempt: 1, max_attempts: 3,
  deadline: "2026-11-20T00:00:00Z", created_at: "2026-11-20T00:00:00Z", finished_at: "2026-11-20T00:00:00Z", diagnostic: null, solutions, program_tests: null,
});

describe("workshop solutions", () => {
  it("shows the expected verdicts next to the result of the newest build that judged the solutions", async () => {
    // The newest build failed before judging, so the one before it holds the results.
    const builds = [
      build(4, null),
      build(3, [{name: "wrong", verdict: "OK", test: null, time_ms: 15, points: null, mismatch: "passed every test, expected WA"}]),
    ];
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      (input: RequestInfo | URL) => {
        const url = new URL(String(input), "http://localhost");
        if (url.pathname === "/api/languages") {
          return Promise.resolve(Response.json({languages: [{id: "cpp20", name: "C++20", extensions: ["cpp"], syntax: "cpp"}]}));
        }
        if (url.pathname === "/api/problems/problem/builds") {
          return Promise.resolve(Response.json(builds));
        }
        return Promise.resolve(Response.json(draft));
      },
      {preconnect: () => {}},
    ));
    restore = () => fetch.mockRestore();
    const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="solution" />);
    await waitFor(() => expect(view.getByText(/Сборка ревизии 3: OK, 15 мс/)).toBeTruthy());
    expect(view.getByText(/не совпадает с ожидаемым \(passed every test, expected WA\)/)).toBeTruthy();
    expect(Array.from(view.container.querySelectorAll(".mantine-Pill-label"), (pill) => pill.textContent)).toEqual(["WA — неправильный ответ"]);
  });
});
