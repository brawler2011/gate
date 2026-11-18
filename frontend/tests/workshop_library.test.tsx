import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopLibTab} from "@/components/workshop/WorkshopLibTab";

import {cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

import type {ProblemDraft} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const SYSTEM = "#define VERSION \"0.9.51\"\n";

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker: null, interactor: null, validators: [], generators: [],
  solutions: [], main_solution_id: null, statements: [], assets: [], subtasks: [], tests: [],
  library: [{id: "lib", name: "testlib.h", content: "", auto_update: true}],
};

describe("workshop library", () => {
  it("turning auto-update off saves an editable copy of the system file", async () => {
    const puts: ProblemDraft[] = [];
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      (input: RequestInfo | URL, init?: RequestInit) => {
        const url = new URL(String(input), "http://localhost");
        if (url.pathname === "/api/lib") {
          const file = {name: "testlib.h", extensions: ["cpp"], version: "0.9.51", commit: "c", content: SYSTEM};
          return Promise.resolve(Response.json({files: [file]}));
        }
        if (init?.method === "PUT") {
          const saved = JSON.parse(String(init.body)) as ProblemDraft;
          puts.push(saved);
          return Promise.resolve(Response.json({...saved, revision: 4}));
        }
        return Promise.resolve(Response.json(draft));
      },
      {preconnect: () => {}},
    ));
    restore = () => fetch.mockRestore();

    const view = renderWithProviders(<WorkshopLibTab problemId="problem" />);
    const button = await view.findByRole("button", {name: "Отключить автообновление"});
    await waitFor(() => expect(button.hasAttribute("disabled")).toBe(false));
    fireEvent.click(button);
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0].library).toEqual([{id: "lib", name: "testlib.h", content: SYSTEM, auto_update: false}]);
    // The copy is an ordinary file the author edits.
    await waitFor(() => expect(view.getByRole("button", {name: "Сохранить"})).toBeTruthy());
  });
});
