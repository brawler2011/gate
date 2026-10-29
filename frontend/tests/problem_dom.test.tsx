import {MantineProvider} from "@mantine/core";
import {afterAll, afterEach, beforeAll, describe, expect, it} from "bun:test";
import React from "react";
import {renderToString} from "react-dom/server";

import {Problem} from "@/components/problems/Problem";

import {cleanup, renderWithProviders, setupDOMEnvironment} from "./test-utils";

describe("Problem Component DOM baseline test", () => {
  let cleanupDOM: () => void;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });

  afterEach(cleanup);

  afterAll(() => {
    cleanupDOM();
  });

  const sampleProblem = {
    id: "prob-1",
    title: "A + B Problem",
    time_limit: 1000,
    memory_limit: 256,
    legend_html: "Calculate $A + B$",
    input_format_html: "Two integers",
    output_format_html: "Single sum integer",
    notes_html: "Note 1",
    scoring_html: "100 points",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    samples: [
      {
        input: "2 2",
        output: "4",
      },
    ],
  };

  it("renders problem title, letter and limits correctly", () => {
    const {container} = renderWithProviders(
      <Problem
        problem={sampleProblem}
        letter="A"
        problemId="prob-1"
        orgLogin="testorg"
      />
    );

    expect(container.textContent).toContain("A. A + B Problem");
    expect(container.textContent).toContain("1 сек");
    expect(container.textContent).toContain("256 МБ");
    expect(container.textContent).toContain("2 2");
    expect(container.textContent).toContain("4");
    expect(container.textContent).toContain("Calculate");
  });

  it("hides all statements and samples for participants while preserving manager access", () => {
    const view = renderWithProviders(<Problem problem={sampleProblem} hideStatements />);
    expect(view.getByText("Условия задач скрыты")).toBeTruthy();
    expect(view.container.textContent).not.toContain("Calculate");
    expect(view.container.querySelector("pre")).toBeNull();
    view.rerender(<Problem problem={sampleProblem} hideStatements isManager orgLogin="school" />);
    expect(view.queryByText("Условия задач скрыты")).toBeNull();
    expect(view.container.textContent).toContain("Calculate");
    expect(view.container.querySelector("a")?.getAttribute("href")).toBe("/school/problems/prob-1");
  });

  it("renders all samples verbatim and omits empty sections", () => {
    const view = renderWithProviders(<Problem problem={{...sampleProblem,
      input_format_html: "", output_format_html: "", notes_html: "",
      samples: [{input: "1 2\n3", output: "6\n"}, {input: "0 0", output: "0"}],
    }} />);
    expect(Array.from(view.container.querySelectorAll("pre")).map((node) => node.textContent)).toEqual([
      "1 2\n3", "6\n", "0 0", "0",
    ]);
    expect(view.queryByRole("heading", {name: "Входные данные"})).toBeNull();
    expect(view.queryByText("Примечание")).toBeNull();
  });

  it("server renders the decomposed content without requiring client effects", () => {
    const html = renderToString(<MantineProvider><Problem problem={sampleProblem} letter="B" /></MantineProvider>);
    expect(html.replaceAll("<!-- -->", "")).toContain("B. A + B Problem");
    expect(html).toContain("Calculate");
    expect(html).toContain("2 2");
  });

  it("renders and replaces statement math after client updates", () => {
    const view = renderWithProviders(<Problem problem={{...sampleProblem, legend_html: "$x^2$"}} />);
    expect(view.container.querySelector(".katex")).toBeTruthy();
    view.rerender(<Problem problem={{...sampleProblem, legend_html: "$y^3$"}} />);
    expect(view.container.querySelector(".katex")).toBeTruthy();
    expect(view.container.querySelector(".katex")?.textContent).toContain("y");
  });

});
