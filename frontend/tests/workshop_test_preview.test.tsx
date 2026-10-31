import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {loadTestState, renumberTests} from "@/components/workshop/tests/testsConfig";
import {TestsListTable} from "@/components/workshop/tests/TestsListTable";
import {WorkshopTestsManager} from "@/components/workshop/tests/WorkshopTestsManager";
import {api} from "@/lib/api";

import {cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

describe("workshop file preview identity", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  let restore: (() => void) | undefined;
  afterEach(() => {
    cleanup(); restore?.();
  });
  afterAll(() => {
    cleanupDOM();
  });

  for (const [filename, outFilename, ordinal] of [["edge.in", "edge.ans", 1], ["03.in", "03.out", 3]] as const) {
    for (const selected of [filename, outFilename]) {
      it(`opens and saves ${selected} using its filename after renumbering`, async () => {
        const read = spyOn(api, "GET").mockImplementation(async (path: string, _init?: unknown) => {
          if (path === "/problems/{problemId}/tests/{name}") {
            return {data: new Blob(["original"]), response: new Response()} as never;
          }
          return {response: new Response()} as never;
        });
        const write = spyOn(api, "PUT").mockImplementation(async () => {
          return {data: {message: "saved"}, response: new Response()} as never;
        });
        restore = () => {
          read.mockRestore(); write.mockRestore();
        };
        const {loadedTests} = loadTestState({tests: [{ordinal, filename, method: "manual"}]}, [filename, outFilename]);
        const view = renderWithProviders(<TestsListTable
          problemId="problem" tests={renumberTests(loadedTests)} subtasks={[]} generators={[]}
          onChangeTests={() => {}} onChangeSubtasks={() => {}} onAddTest={() => {}} onDeleteSelectedTests={() => {}}
        />);
        fireEvent.click(view.getByText(`${selected} ✓`));
        await waitFor(() => expect(read).toHaveBeenCalledWith(
          "/problems/{problemId}/tests/{name}",
          expect.objectContaining({params: {path: {problemId: "problem", name: selected}}}),
        ));
        const editor = await view.findByLabelText("Содержимое файла");
        await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe("original"));
        fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
        await waitFor(() => expect(write).toHaveBeenCalledTimes(1));
        const writeCall = write.mock.calls[0];
        expect((writeCall[1] as {params?: {path?: {name?: string}}})?.params?.path?.name).toBe(selected);
        const body = (writeCall[1] as {body?: unknown})?.body;
        expect(await (body instanceof Blob ? body.text() : String(body))).toBe("original");
      });
    }
  }
  for (const selected of ["nested/edge.in", "nested/edge.ans"]) {
    it(`loads the test API listing and opens/saves ${selected} through the manager`, async () => {
      const files: Record<string, string> = {"nested/edge.in": "nested input", "nested/edge.ans": "nested answer"};
      const config = {
        groups: [{ordinal: 1, name: "samples", points: 0, points_policy: "complete", depends_on: [], tests: [1, 1]}],
        tests: [{ordinal: 1, filename: "nested/edge", method: "manual", is_sample: true}],
      };
      const read = spyOn(api, "GET").mockImplementation(async (path: string, init?: unknown) => {
        if (path === "/problems/{problemId}/limits") {
          return {
            data: {problem_type: "pass-fail", time_limit_ms: 1000, memory_limit_mb: 256},
            response: new Response(),
          } as never;
        }
        if (path === "/problems/{problemId}/generators") {
          return {data: {files: []}, response: new Response()} as never;
        }
        if (path === "/problems/{problemId}/submissions") {
          return {data: {files: []}, response: new Response()} as never;
        }
        if (path === "/problems/{problemId}/tests") {
          return {
            data: {
              files: Object.entries(files).map(([name, content]) => ({
                path: `tests/${name}`, size: content.length, is_directory: false,
              })),
            },
            response: new Response(),
          } as never;
        }
        if (path === "/problems/{problemId}/tests/{name}") {
          const name = (init as {params?: {path?: {name?: string}}})?.params?.path?.name;
          let content = "";
          if (name === "tests.json") {
            content = JSON.stringify(config);
          } else if (name && files[name]) {
            content = files[name];
          }
          return {data: new Blob([content]), response: new Response()} as never;
        }
        return {response: new Response()} as never;
      });
      const write = spyOn(api, "PUT").mockImplementation(async () => {
        return {data: {message: "saved"}, response: new Response()} as never;
      });
      restore = () => {
        read.mockRestore(); write.mockRestore();
      };
      const view = renderWithProviders(<WorkshopTestsManager problemId={`nested-${selected.endsWith("in") ? "input" : "answer"}`} />);
      fireEvent.click(await view.findByText(`${selected} ✓`));
      await waitFor(() => expect(
        read.mock.calls.some(
          (call: unknown[]) =>
            call[0] === "/problems/{problemId}/tests/{name}" &&
            (call[1] as {params?: {path?: {name?: string}}})?.params?.path?.name === selected,
        ),
      ).toBe(true));
      const editor = await view.findByLabelText("Содержимое файла");
      await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe(files[selected]));
      fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
      await waitFor(() => expect(write).toHaveBeenCalledTimes(1));
      const writeCall = write.mock.calls[0];
      expect((writeCall[1] as {params?: {path?: {name?: string}}})?.params?.path?.name).toBe(selected);
      const body = (writeCall[1] as {body?: unknown})?.body;
      expect(await (body instanceof Blob ? body.text() : String(body))).toBe(files[selected]);
    });
  }

});
