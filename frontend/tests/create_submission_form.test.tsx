import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, mock} from "bun:test";
import React from "react";

import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";

import {act, cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

const CATALOG = [
  {id: "cpp23", name: "C++23 (GCC 15)", extensions: ["cpp", "cc", "cxx"], syntax: "cpp"},
  {id: "cpp20", name: "C++20 (GCC 15)", extensions: ["cpp", "cc", "cxx"], syntax: "cpp"},
  {id: "python3", name: "Python 3.14 (CPython)", extensions: ["py"], syntax: "python"},
  {id: "go", name: "Go 1.26", extensions: ["go"], syntax: "go"},
];

describe("submission language selection", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });
  afterAll(() => {
    cleanupDOM();
  });

  const renderForm = (languages = CATALOG, result: number | null = null) => {
    const onSubmit = mock(async (_submission: FormData, _language: string) => result);
    const view = renderWithProviders(<CreateSubmissionForm languages={languages} onSubmit={onSubmit} />);
    const language = view.getByRole("textbox", {name: "Язык программирования"}) as HTMLInputElement;
    const form = view.getByText("Отправить решение").closest("form")!;
    const dropCode = (source: string) => {
      fireEvent.drop(form.firstElementChild!, {dataTransfer: {getData: () => source, files: []}});
    };
    const attach = (file: File, drop = false) => {
      if (drop) {
        fireEvent.drop(form.firstElementChild!, {dataTransfer: {getData: () => "", files: [file]}});
      } else {
        fireEvent.change(view.container.querySelector('input[type="file"]')!, {target: {files: [file]}});
      }
    };
    const select = (name: string) => {
      fireEvent.click(language);
      fireEvent.click(view.getByRole("option", {name}));
    };
    return {view, onSubmit, language, form, dropCode, attach, select};
  };

  it("restores the manual selection for pasted code without inspecting its contents", async () => {
    const first = renderForm();
    expect(first.language.value).toBe(CATALOG[0].name);
    first.select(CATALOG[2].name);
    first.view.unmount();

    const {language, dropCode, form, onSubmit} = renderForm();
    expect(language.value).toBe(CATALOG[2].name);
    dropCode("package main\nfunc main() {}");
    await act(async () => {
      fireEvent.submit(form); 
    });
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][1]).toBe("python3");
    expect(onSubmit.mock.calls[0][0].get("submission")).toBe("package main\nfunc main() {}");
  });

  it("falls back to the first allowed configuration when the saved one is unavailable", () => {
    const first = renderForm();
    first.select(CATALOG[2].name);
    first.view.unmount();
    const {view, language} = renderForm([CATALOG[1], CATALOG[3]]);
    expect(language.value).toBe(CATALOG[1].name);
    fireEvent.click(language);
    expect(view.queryByRole("option", {name: CATALOG[2].name})).toBeNull();
    expect(view.queryByRole("option", {name: "Автоопределение"})).toBeNull();
  });

  it("selects and persists the uploaded file's extension despite conflicting contents", async () => {
    const first = renderForm();
    const file = new File(["#include <iostream>\nint main() {}"], "solution.PY");
    first.attach(file);
    expect(first.language.value).toBe(CATALOG[2].name);
    await act(async () => {
      fireEvent.submit(first.form); 
    });
    await waitFor(() => expect(first.onSubmit).toHaveBeenCalledTimes(1));
    const [submitted, language] = first.onSubmit.mock.calls[0];
    expect(language).toBe("python3");
    expect(await (submitted.get("submission") as File).text()).toBe(await file.text());
    first.view.unmount();
    expect(renderForm().language.value).toBe(CATALOG[2].name);
  });

  it("prefers a saved matching configuration, otherwise uses catalog priority for dropped files", () => {
    const {language, select, attach} = renderForm();
    select(CATALOG[1].name);
    attach(new File([""], "solution.cc"), true);
    expect(language.value).toBe(CATALOG[1].name);
    select(CATALOG[2].name);
    attach(new File([""], "solution.cpp"), true);
    expect(language.value).toBe(CATALOG[0].name);
  });

  it("keeps the saved language for unknown extensions and allows overriding the file language", async () => {
    const {language, select, attach, form, onSubmit} = renderForm();
    select(CATALOG[3].name);
    attach(new File(["def solve(): pass"], "solution.txt"));
    expect(language.value).toBe(CATALOG[3].name);
    attach(new File(["int main() {}"], "solution.unknown"), true);
    expect(language.value).toBe(CATALOG[3].name);
    attach(new File(["print(1)"], "solution.py"));
    select(CATALOG[1].name);
    await act(async () => {
      fireEvent.submit(form); 
    });
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit.mock.calls[0][1]).toBe("cpp20");
  });

  it("clears a successful submission while preserving the language for the next one", async () => {
    const {view, language, attach, form, dropCode, onSubmit} = renderForm(CATALOG, 42);
    attach(new File(["print(1)"], "solution.py"));
    await act(async () => {
      fireEvent.submit(form); 
    });
    await waitFor(() => expect(view.queryByText("Прикреплен файл: solution.py")).toBeNull());
    expect(language.value).toBe(CATALOG[2].name);
    dropCode("print(2)");
    await act(async () => {
      fireEvent.submit(form); 
    });
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2));
    expect(onSubmit.mock.calls[1][1]).toBe("python3");
    expect(onSubmit.mock.calls[1][0].get("submission")).toBe("print(2)");
  });

  it("keeps the active selection when another tab saves an unavailable language", () => {
    const {language, select} = renderForm(CATALOG.slice(0, 2));
    select(CATALOG[1].name);
    act(() => {
      window.localStorage.setItem("submission-language", JSON.stringify("go"));
      window.dispatchEvent(new window.StorageEvent("storage", {
        key: "submission-language",
        newValue: JSON.stringify("go"),
        storageArea: window.localStorage,
      }));
    });
    expect(language.value).toBe(CATALOG[1].name);
    expect(window.localStorage.getItem("submission-language")).toBe(JSON.stringify("go"));
  });

  it("does not submit without an available language", () => {
    const {form, dropCode, onSubmit} = renderForm([]);
    dropCode("print(1)");
    fireEvent.submit(form);
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
