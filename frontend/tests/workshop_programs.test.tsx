import "./setup-dom";

import {afterEach, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {WorkshopComponentsTab} from "@/components/workshop/WorkshopComponentsTab";

import {act, cleanup, fireEvent, renderWithProviders, waitFor} from "./test-utils";

import type {ProblemDraft} from "@/contracts/core/v1/schema";

let restore: (() => void) | undefined;
afterEach(() => {
  cleanup();
  restore?.();
});

const generator = (name: string, source: string) => ({name, language: "cpp20", source});

const draft: ProblemDraft = {
  problem_id: "problem", revision: 3, title: "A", short_name: "a", type: "pass-fail", time_limit_ms: 1000,
  memory_limit_mb: 256, input_file: "", output_file: "", max_score: null, checker: null, interactor: null, validators: [],
  generators: [generator("first", "FIRST SOURCE"), generator("second", "SECOND SOURCE")], solutions: [],
  main_solution: null, statements: [], assets: [], subtasks: [], tests: [], library: [],
};

let puts = 0;

const mockApi = (stored: ProblemDraft, putStatus = 200): void => {
  puts = 0;
  const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(
    (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), "http://localhost");
      if (url.pathname === "/api/languages") {
        return Promise.resolve(Response.json({languages: [{id: "cpp20", name: "C++20", extensions: ["cpp"], syntax: "cpp"}]}));
      }
      if (init?.method === "PUT") {
        puts++;
        if (putStatus !== 200) {
          return Promise.resolve(Response.json({message: "unavailable"}, {status: putStatus}));
        }
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

  // happy-dom requires invoking React's change handler directly.
  const type = async (element: Element, text: string): Promise<void> => {
    const propsKey = Object.keys(element).find((key) => key.startsWith("__reactProps")) as string;
    const props = (element as unknown as Record<string, {onChange: (event: unknown) => void}>)[propsKey];
    await act(async () => props.onChange({currentTarget: {value: text}, target: {value: text}}));
  };

  for (const [status, name] of [[200, "renamed"], [500, "second"]] as const) {
    it(`a rename keeps unsaved source when the save answers ${status}`, async () => {
      mockApi(draft, status);
      const view = renderWithProviders(<WorkshopComponentsTab problemId="problem" role="generator" />);
      await waitFor(() => expect(view.container.querySelector("textarea")?.value).toBe("FIRST SOURCE"));
      fireEvent.click(view.getByText("second"));
      await waitFor(() => expect(view.container.querySelector("textarea")?.value).toBe("SECOND SOURCE"));
      await type(view.container.querySelector("textarea") as Element, "UNSAVED EDIT");
      await waitFor(() => expect(view.getByRole("button", {name: "Сохранить"}).hasAttribute("disabled")).toBe(false));
      await type(view.getByLabelText("Название"), "renamed");
      await waitFor(() => expect(view.getByRole("button", {name: "Переименовать"}).hasAttribute("disabled")).toBe(false));
      fireEvent.click(view.getByRole("button", {name: "Переименовать"}));
      await waitFor(() => expect(puts).toBe(1));
      const names = () => Array.from(view.container.querySelectorAll(".mantine-NavLink-label"), (label) => label.textContent);
      await waitFor(() => expect(names()).toEqual(["first", name]));
      expect(view.container.querySelector("textarea")?.value).toBe("UNSAVED EDIT");
      expect(view.getByRole("button", {name: "Сохранить"}).hasAttribute("disabled")).toBe(false);
    });
  }
});
