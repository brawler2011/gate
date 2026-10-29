import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {loadTestState, renumberTests} from "@/components/workshop/tests/testsConfig";
import {TestsListTable} from "@/components/workshop/tests/TestsListTable";
import {WorkshopTestsManager} from "@/components/workshop/tests/WorkshopTestsManager";
import {DefaultService} from "@/contracts/core/v1";
import {CancelablePromise} from "@/contracts/core/v1/core/CancelablePromise";

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
        const read = spyOn(DefaultService.prototype, "getProblemTestFile").mockResolvedValue(new Blob(["original"]));
        const write = spyOn(DefaultService.prototype, "updateProblemTestFile").mockResolvedValue({message: "saved"});
        restore = () => {
          read.mockRestore(); write.mockRestore();
        };
        const {loadedTests} = loadTestState({tests: [{ordinal, filename, method: "manual"}]}, [filename, outFilename]);
        const view = renderWithProviders(<TestsListTable
          problemId="problem" tests={renumberTests(loadedTests)} subtasks={[]} generators={[]}
          onChangeTests={() => {}} onChangeSubtasks={() => {}} onAddTest={() => {}} onDeleteSelectedTests={() => {}}
        />);
        fireEvent.click(view.getByText(`${selected} ✓`));
        await waitFor(() => expect(read).toHaveBeenCalledWith({problemId: "problem", name: selected}));
        const editor = await view.findByLabelText("Содержимое файла");
        await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe("original"));
        fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
        await waitFor(() => expect(write).toHaveBeenCalledTimes(1));
        expect(write.mock.calls[0][0].name).toBe(selected);
        expect(await write.mock.calls[0][0].requestBody.text()).toBe("original");
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
      const read = spyOn(DefaultService.prototype, "getProblemTestFile").mockImplementation(({name}) =>
        new CancelablePromise((resolve) => resolve(new Blob([name === "tests.json" ? JSON.stringify(config) : files[name]])))
      );
      const write = spyOn(DefaultService.prototype, "updateProblemTestFile").mockResolvedValue({message: "saved"});
      const mocks = [read, write,
        spyOn(DefaultService.prototype, "listProblemGenerators").mockResolvedValue({files: []}),
        spyOn(DefaultService.prototype, "listProblemWorkshopSubmissions").mockResolvedValue({files: []}),
        spyOn(DefaultService.prototype, "listProblemTests").mockResolvedValue({files: Object.entries(files).map(
          ([name, content]) => ({path: `tests/${name}`, size: content.length, is_directory: false})
        )}),
        spyOn(DefaultService.prototype, "getProblemLimits").mockResolvedValue({
          problem_type: "pass-fail", time_limit_ms: 1000, memory_limit_mb: 256,
        }),
      ];
      restore = () => {
        mocks.forEach((mock) => mock.mockRestore());
      };
      const view = renderWithProviders(<WorkshopTestsManager problemId={`nested-${selected.endsWith("in") ? "input" : "answer"}`} />);
      fireEvent.click(await view.findByText(`${selected} ✓`));
      await waitFor(() => expect(read.mock.calls.some(([args]) => args.name === selected)).toBe(true));
      const editor = await view.findByLabelText("Содержимое файла");
      await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe(files[selected]));
      fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
      await waitFor(() => expect(write).toHaveBeenCalledTimes(1));
      expect(write.mock.calls[0][0].name).toBe(selected);
      expect(await write.mock.calls[0][0].requestBody.text()).toBe(files[selected]);
    });
  }

});
