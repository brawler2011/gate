import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {loadTestState, renumberTests} from "@/components/workshop/tests/testsConfig";
import {TestsListTable} from "@/components/workshop/tests/TestsListTable";
import {DefaultService} from "@/contracts/core/v1";

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
});
