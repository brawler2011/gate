import "./setup-dom";

import {Notifications, notifications} from "@mantine/notifications";
import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";

import {WorkshopTestsManager} from "@/components/workshop/tests/WorkshopTestsManager";

import {act, cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

describe("workshop file editing", () => {
  let cleanupDOM: () => void;
  let restoreFetch: (() => void) | undefined;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(() => {
    cleanup(); notifications.clean(); restoreFetch?.();
  });
  afterAll(() => {
    cleanupDOM();
  });

  const setupFiles = (filename: string, answer: string, ordinal: number, failSave = false) => {
    const files: Record<string, string> = {[filename]: "original input", [answer]: "original answer", "unrelated.in": "untouched"};
    const config = {
      groups: [{ordinal: 1, name: "samples", points: 0, points_policy: "complete", depends_on: [], tests: [ordinal, ordinal]}],
      tests: [{ordinal, filename, method: "manual", is_sample: true}],
    };
    const prefix = "/api/problems/problem/tests";
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = new URL(String(input), "http://localhost").pathname;
      if (path === "/api/problems/problem/limits") {
        return Response.json({problem_type: "pass-fail", time_limit_ms: 1000, memory_limit_mb: 256});
      }
      if (path === "/api/problems/problem/generators" || path === "/api/problems/problem/submissions") {
        return Response.json({files: []});
      }
      if (path === prefix) {
        return Response.json({files: Object.entries(files).map(([name, content]) => ({path: `tests/${name}`, size: content.length, is_directory: false}))});
      }
      if (path.startsWith(prefix + "/")) {
        const name = decodeURIComponent(path.slice(prefix.length + 1));
        if (name === "tests.json") {
          return Response.json(config);
        }
        if (!(name in files)) {
          return Response.json({message: "Missing file"}, {status: 404});
        }
        if (init?.method === "PUT") {
          if (failSave) {
            return Response.json({message: "Save failed"}, {status: 500});
          }
          files[name] = await new Response(init.body).text();
          return Response.json({message: "Saved"});
        }
        return new Response(files[name], {headers: {"Content-Type": "application/octet-stream"}});
      }
      throw new Error(`Unexpected request ${path}`);
    }, {preconnect: () => {}}));
    restoreFetch = () => fetch.mockRestore();
    return files;
  };

  for (const [filename, answer, ordinal] of [
    ["edge.in", "edge.ans", 1], ["03.in", "03.out", 3], ["nested/edge.in", "nested/edge.ans", 1],
  ] as const) {
    for (const selected of [filename, answer]) {
      it(`persists edits to ${selected} without overwriting other files`, async () => {
        const files = setupFiles(filename, answer, ordinal);
        const original = {...files};
        const view = renderWithProviders(<WorkshopTestsManager problemId="problem" />);
        fireEvent.click(await view.findByText(`${selected} ✓`));
        const editor = await view.findByLabelText("Содержимое файла") as HTMLTextAreaElement;
        await waitFor(() => expect(editor.value).toBe(original[selected]));
        fireEvent.focusIn(editor);
        fireEvent.input(editor, {target: {value: "edited content\n"}});
        fireEvent.keyUp(editor, {key: "Enter"});
        await act(async () => {
          fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
        });
        await waitFor(() => expect(view.queryByRole("dialog")).toBeNull());
        expect(files).toEqual({...original, [selected]: "edited content\n"});
        fireEvent.click(await view.findByText(`${selected} ✓`));
        const reopened = await view.findByLabelText("Содержимое файла") as HTMLTextAreaElement;
        await waitFor(() => expect(reopened.value).toBe("edited content\n"));
      });
    }
  }

  it("retains unsaved edits and the original file after a save failure", async () => {
    const files = setupFiles("edge.in", "edge.ans", 1, true);
    const original = {...files};
    const view = renderWithProviders(<><Notifications /><WorkshopTestsManager problemId="problem" /></>);
    fireEvent.click(await view.findByText("edge.in ✓"));
    const editor = await view.findByLabelText("Содержимое файла") as HTMLTextAreaElement;
    await waitFor(() => expect(editor.value).toBe("original input"));
    fireEvent.focusIn(editor);
    fireEvent.input(editor, {target: {value: "unsaved changes"}});
    fireEvent.keyUp(editor, {key: "s"});
    await act(async () => {
      fireEvent.click(view.getByRole("button", {name: "Сохранить"}));
    });
    await view.findByText("Save failed");
    expect(view.getByRole("dialog")).toBeTruthy();
    expect(editor.value).toBe("unsaved changes");
    expect(files).toEqual(original);
  });
});
