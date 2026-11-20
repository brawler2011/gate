import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopStatementTab} from "@/components/workshop/WorkshopStatementTab";

import {cleanup, renderWithProviders, waitFor} from "./test-utils";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const draft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, input_file: "", output_file: "", max_score: null, checker: null, interactor: null, validators: [], generators: [],
  solutions: [], main_solution: null, library: [], subtasks: [], tests: [],
  assets: [{name: "график.png", object_id: "graph"}],
  statements: [{language: "ru", legend: "![граф](./график.png#center)", input: "", output: "", notes: "", interaction: "", scoring: ""}],
};

describe("workshop statement tab", () => {
  it("previews an asset referenced by its Unicode name as participants see it", async () => {
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
      () => Promise.resolve(Response.json(draft)),
      {preconnect: () => {}},
    ));
    restore = () => fetch.mockRestore();
    const view = renderWithProviders(<WorkshopStatementTab problemId="problem" />);
    const image = await waitFor(() => view.container.querySelector("img[alt='граф']") ?? Promise.reject(new Error("no image")));
    expect(image.getAttribute("src")).toBe("/api/problems/problem/draft/objects/graph");
  });
});
