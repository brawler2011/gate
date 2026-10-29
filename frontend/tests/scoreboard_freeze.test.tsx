import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {ContestMonitorTable} from "@/components/contests/ContestMonitorTable";
import {env} from "@/lib/env";
import {submissionsWsManager} from "@/lib/submissionsWsManager";

import {cleanup, renderWithProviders, setupDOMEnvironment, within} from "./test-utils";

import type {ScoreboardProblemResultModel, ScoreboardResponseModel} from "@/contracts/core/v1";

const scoreboard = (results: ScoreboardProblemResultModel[]): ScoreboardResponseModel => ({
  contest_id: "contest", contest_login: "round", organization_login: "org", is_frozen: true,
  penalty_per_attempt: 20,
  problems: [{problem_id: "p", title: "A+B", ordinal: 1, short_name: "A"}],
  items: results.map((result, index) => ({
    user_id: `u${index}`, username: `User ${index}`, problems_solved: Number(result.solved),
    total_penalty: 40, problem_results: [result],
  })),
});

const result = (solved: boolean, failed_attempts: number, pending_attempts: number): ScoreboardProblemResultModel => ({
  problem_id: "p", solved, failed_attempts, pending_attempts,
});

describe("Production scoreboard freeze presentation", () => {
  let cleanupDOM: () => void;
  let remove: ReturnType<typeof spyOn>;
  let add: ReturnType<typeof spyOn>;
  let wsUrl: ReturnType<typeof spyOn>;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
    add = spyOn(submissionsWsManager, "addListener").mockReturnValue(1);
    remove = spyOn(submissionsWsManager, "removeListener").mockImplementation(() => {});
    wsUrl = spyOn(env, "getWebSocketUrl").mockReturnValue("ws://localhost/ws");
  });
  afterEach(cleanup);
  afterAll(() => {
    add.mockRestore(); remove.mockRestore(); wsUrl.mockRestore(); cleanupDOM();
  });

  it.each([
    [false, 0, 3, "?3"],
    [false, 2, 1, "-2 1?"],
    [true, 0, 2, "+ 2?"],
    [true, 3, 2, "+3 2?"],
    [true, 0, 0, "+"],
    [true, 3, 0, "+3"],
    [false, 2, 0, "-2"],
  ])("renders solved=%s failed=%s pending=%s as %s", (solved, failed, pending, label) => {
    const view = renderWithProviders(<ContestMonitorTable contestId="contest" orgLogin="org" contestLogin="round"
      initialScoreboard={scoreboard([result(solved as boolean, failed as number, pending as number)])} />);
    const cell = within(view.getAllByRole("row")[1]).getAllByRole("cell")[4];
    expect(cell.textContent).toBe(label as string);
    expect(view.getByRole("link", {name: "A"}).getAttribute("href")).toBe("/org/round/A");
  });

  it("counts frozen attempts without inflating solved totals", () => {
    const view = renderWithProviders(<ContestMonitorTable contestId="contest" orgLogin="org" contestLogin="round"
      initialScoreboard={scoreboard([result(true, 1, 2), result(false, 0, 3), result(false, 0, 0)])} />);
    const summary = (name: string) => within(view.getByText(name).closest("tr")!).getAllByRole("cell")[4].textContent;
    expect(summary("Сдали")).toBe("1");
    expect(summary("Пытались")).toBe("2");

  });

  it("renders unattempted and missing results as empty cells and hides manager controls", () => {
    const snapshot = scoreboard([result(false, 0, 0), result(false, 0, -1)]);
    snapshot.items[1].problem_results = [];
    const view = renderWithProviders(<ContestMonitorTable contestId="contest" orgLogin="org" contestLogin="round" initialScoreboard={snapshot} />);
    for (const row of view.getAllByRole("row").slice(1, 3)) {
      expect(within(row).getAllByRole("cell")[4].textContent).toBe("");
    }
    expect(view.queryByRole("switch")).toBeNull();
  });
});
