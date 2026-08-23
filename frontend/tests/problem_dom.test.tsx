import {afterAll, beforeAll, describe, expect, it} from "bun:test";
import React from "react";

import {Problem} from "@/components/problems/Problem";

import {renderWithProviders, setupDOMEnvironment} from "./test-utils";

describe("Problem Component DOM baseline test", () => {
  let cleanupDOM: () => void;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });

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
});
