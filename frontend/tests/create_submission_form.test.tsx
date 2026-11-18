import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, mock} from "bun:test";
import React from "react";

import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";

import {cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

// Two configurations accept .cpp; detected C++ selects the first allowed one.
const CATALOG = [
  {id: "cpp17", name: "C++17 (GCC 15)", extensions: ["cpp", "cc", "cxx"], syntax: "cpp"},
  {id: "cpp20", name: "C++20 (GCC 15)", extensions: ["cpp", "cc", "cxx"], syntax: "cpp"},
  {id: "python3", name: "Python 3.14 (CPython)", extensions: ["py"], syntax: "python"},
  {id: "go", name: "Go 1.26", extensions: ["go"], syntax: "go"},
];
const AUTO = "Автоопределение";

describe("submission language selection", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(cleanup);
  afterAll(() => {
    cleanupDOM();
  });

  const renderForm = (languages = CATALOG) => {
    const onSubmit = mock(async (_submission: FormData, _language: string) => null);
    const view = renderWithProviders(<CreateSubmissionForm languages={languages} onSubmit={onSubmit} />);
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
    expect(language.value).toBe(AUTO);
    drop("#include <iostream>\nint main() {}");
    await waitFor(() => expect(language.value).toBe("C++17 (GCC 15)"));
    select("Python 3.14 (CPython)");
    drop("package main\nfunc main() {}");
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(language.value).toBe("Python 3.14 (CPython)");
    select(AUTO);
    await waitFor(() => expect(language.value).toBe("Go 1.26"));
  });

  it("debounces rapid source replacements", async () => {
    const {language, drop} = renderForm();
    drop("#include <iostream>\nint main() {}");
    drop("def solve():\n  return 1");
    expect(language.value).toBe(AUTO);
    await waitFor(() => expect(language.value).toBe("Python 3.14 (CPython)"));
  });

  it("preserves manual selection of the already detected language", async () => {
    const {language, drop, select} = renderForm();
    drop("#include <iostream>\nint main() {}");
    await waitFor(() => expect(language.value).toBe("C++17 (GCC 15)"));
    select("C++17 (GCC 15)");
    drop("package main\nfunc main() {}");
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(language.value).toBe("C++17 (GCC 15)");
  });

  it("does not submit unresolved Auto or stale detection after replacing code", async () => {
    const {view, onSubmit, language, drop} = renderForm();
    drop("func main() {}");
    await waitFor(() => expect(language.value).toBe("Go 1.26"));
    drop("x = 1");
    fireEvent.submit(view.getByText("Отправить решение").closest("form")!);
    expect(onSubmit).not.toHaveBeenCalled();
    await waitFor(() => expect(language.value).toBe(AUTO));
    expect(view.getByText("Не удалось определить язык. Выберите его вручную.")).toBeTruthy();
  });

  it("detects uploaded contents instead of trusting the filename and submits the actual language", async () => {
    const {view, onSubmit, language} = renderForm();
    const file = new File(["#include <iostream>\nint main() {}"], "solution.py");
    fireEvent.change(view.container.querySelector('input[type="file"]')!, {target: {files: [file]}});
    await waitFor(() => expect(language.value).toBe("C++17 (GCC 15)"));
    fireEvent.submit(view.getByText("Отправить решение").closest("form")!);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    const [submitted, selectedLanguage] = onSubmit.mock.calls[0];
    expect(selectedLanguage).toBe("cpp17");
    const source = submitted.get("submission");
    expect(source).toBeInstanceOf(File);
    expect(await (source as File).text()).toBe(await file.text());
  });

  it("offers only the allowed configurations and detects among them", async () => {
    const {view, language, drop} = renderForm(CATALOG.filter((option) => option.id !== "cpp17"));
    fireEvent.click(language);
    expect(view.queryByRole("option", {name: "C++17 (GCC 15)"})).toBeNull();
    fireEvent.click(language);
    drop("#include <iostream>\nint main() {}");
    await waitFor(() => expect(language.value).toBe("C++20 (GCC 15)"));
  });
});
