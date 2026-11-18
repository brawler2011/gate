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

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, max_score: null, checker: null, interactor: null, validators: [], generators: [],
  solutions: [], main_solution_id: null, statements: [], assets: [], subtasks: [], tests: [],
  library: [{id: "lib", name: "testlib.h", content: "", auto_update: true}],
};

describe("workshop components", () => {
  it("a new checker waits for the system library and depends on its auto-updated files", async () => {
    let releaseLibrary = () => {};
    const library = new Promise<void>((resolve) => {
      releaseLibrary = resolve;
    });
    const puts: ProblemDraft[] = [];
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = new URL(String(input), "http://localhost");
        if (url.pathname === "/api/languages") {
          return Response.json({languages: [{id: "cpp17", name: "C++17", extensions: ["cpp"], syntax: "cpp"}]});
        }
        if (url.pathname === "/api/lib") {
          await library;
          return Response.json({files: [{name: "testlib.h", extensions: ["cpp"], version: "1", commit: "c", content: ""}]});
        }
        if (init?.method === "PUT") {
          const saved = JSON.parse(String(init.body)) as ProblemDraft;
          puts.push(saved);
          return Response.json({...saved, revision: 4});
        }
        return Response.json(draft);
      },
      {preconnect: () => {}},
    ));
    restore = () => fetch.mockRestore();

    const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="checker" />);
    const create = await view.findByRole("button", {name: "Создать"});
    await waitFor(() => expect(view.getByRole("textbox", {name: "Язык"}).getAttribute("value")).toBe("C++17"));
    expect(create.hasAttribute("disabled")).toBe(true);

    releaseLibrary();
    await waitFor(() => expect(create.hasAttribute("disabled")).toBe(false));
    fireEvent.click(create);
    await waitFor(() => expect(puts).toHaveLength(1));
    expect(puts[0].checker?.dependencies).toEqual(["lib"]);
  });
});
