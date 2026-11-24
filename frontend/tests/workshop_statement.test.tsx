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
  solutions: [], main_solution: null, library: [], testset: {id: "root", name: "", aggregate: "sum", stop_on_failure: false, requires: [], feedback: "complete", checker_args: [], validator_args: [], groups: []}, tests: [],
  assets: [{name: "график.png", object_id: "graph"}],
  statements: [{language: "ru", title: "", legend: "\\includegraphics{график.png}", input: "", output: "", notes: "", interaction: "", scoring: ""}],
};

const respond = (handler: (url: string, init?: RequestInit) => Response) => {
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: unknown, init?: RequestInit) => Promise.resolve(handler(String(input), init)),
    {preconnect: () => {}},
  ));
  restore = () => fetch.mockRestore();
};

describe("workshop statement tab", () => {
  it("previews the edited statement as the server typesets it, with draft images", async () => {
    let previewed: unknown;
    respond((url, init) => {
      if (!url.endsWith("/statement-preview")) {
        return Response.json(draft);
      }
      previewed = JSON.parse(String(init?.body));
      return Response.json({legend: '<p><img src="график.png" alt="граф"/></p>'});
    });
    const view = renderWithProviders(<WorkshopStatementTab problemId="problem" />);
    (await view.findByRole("button", {name: "Обновить предпросмотр"})).click();
    const image = await waitFor(() => view.container.querySelector("img[alt='граф']") ?? Promise.reject(new Error("no image")));
    expect(image.getAttribute("src")).toBe("/api/problems/problem/draft/objects/graph");
    expect(previewed).toEqual(draft.statements[0]);
  });

  it("shows where typesetting failed", async () => {
    respond((url) => url.endsWith("/statement-preview")
      ? Response.json({type: "about:blank", title: "Bad Request", status: 400, code: "format", request_id: "r", detail: "legend section, line 2: \\foo is not supported on the site"}, {status: 400})
      : Response.json(draft));
    const view = renderWithProviders(<WorkshopStatementTab problemId="problem" />);
    (await view.findByRole("button", {name: "Обновить предпросмотр"})).click();
    expect(await view.findByText("legend section, line 2: \\foo is not supported on the site")).toBeTruthy();
  });
});
