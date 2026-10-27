import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, mock} from "bun:test";
import React from "react";

import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";

import {cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

describe("submission language selection", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(cleanup);
  afterAll(() => {
    cleanupDOM();
  });

  const renderForm = () => {
    const onSubmit = mock(async (_submission: FormData, _language: string) => null);
    const view = renderWithProviders(<CreateSubmissionForm onSubmit={onSubmit} />);
    const language = view.getByRole("textbox", {name: "Язык программирования"}) as HTMLInputElement;
    const drop = (source: string) => {
      fireEvent.drop(view.getByText("Отправить решение").closest("form")!.firstElementChild!, {
        dataTransfer: {getData: () => source, files: []},
      });
    };
    const select = (name: string) => {
      fireEvent.click(language);
      fireEvent.click(view.getByRole("option", {name}));
    };
    return {view, onSubmit, language, drop, select};
  };

  it("detects pasted/dropped code, preserves manual override, and can return to Auto", async () => {
    const {language, drop, select} = renderForm();
    expect(language.value).toBe("auto");
    drop("#include <iostream>\nint main() {}");
    await waitFor(() => expect(language.value).toBe("cpp"));
    select("python");
    drop("package main\nfunc main() {}");
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(language.value).toBe("python");
    select("auto");
    await waitFor(() => expect(language.value).toBe("golang"));
  });

  it("debounces rapid source replacements", async () => {
    const {language, drop} = renderForm();
    drop("#include <iostream>\nint main() {}");
    drop("def solve():\n  return 1");
    expect(language.value).toBe("auto");
    await waitFor(() => expect(language.value).toBe("python"));
  });

  it("preserves manual selection of the already detected language", async () => {
    const {language, drop, select} = renderForm();
    drop("#include <iostream>\nint main() {}");
    await waitFor(() => expect(language.value).toBe("cpp"));
    select("cpp");
    drop("package main\nfunc main() {}");
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(language.value).toBe("cpp");
  });

  it("does not submit unresolved Auto or stale detection after replacing code", async () => {
    const {view, onSubmit, language, drop} = renderForm();
    drop("func main() {}");
    await waitFor(() => expect(language.value).toBe("golang"));
    drop("x = 1");
    fireEvent.submit(view.getByText("Отправить решение").closest("form")!);
    expect(onSubmit).not.toHaveBeenCalled();
    await waitFor(() => expect(language.value).toBe("auto"));
    expect(view.getByText("Не удалось определить язык. Выберите его вручную.")).toBeTruthy();
  });

  it("detects uploaded contents instead of trusting the filename and submits the actual language", async () => {
    const {view, onSubmit, language} = renderForm();
    const file = new File(["#include <iostream>\nint main() {}"], "solution.py");
    fireEvent.change(view.container.querySelector('input[type="file"]')!, {target: {files: [file]}});
    await waitFor(() => expect(language.value).toBe("cpp"));
    fireEvent.submit(view.getByText("Отправить решение").closest("form")!);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0]).toEqual([expect.any(FormData), "cpp"]);
  });
});
