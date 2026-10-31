import "./setup-dom";

import {afterAll, beforeAll, describe, expect, it, spyOn} from "bun:test";
import React from "react";

import {ContestMonitorTable} from "@/components/contests/ContestMonitorTable";
import {SubmissionDetailsContent} from "@/components/submissions/SubmissionDetailsContent";
import {SubmissionsList} from "@/components/submissions/SubmissionsList";
import {env} from "@/lib/env";
import {submissionsWsManager} from "@/lib/submissionsWsManager";

import {act, renderWithProviders, setupDOMEnvironment, waitFor} from "./test-utils";

import type {ScoreboardResponseModel, SubmissionModel, SubmissionsListItemModel} from "@/contracts/core/v1/models";

const renderView = (ui: React.ReactElement) => renderWithProviders(
  <>{ui}</>
);

describe("Observer visibility in production components", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterAll(() => {
    cleanupDOM();
  });

  const metadata: SubmissionsListItemModel = {
    id: "sub", user_id: "user", username: "alice", language: 20,
    problem_id: "problem", problem_title: "A+B", position: 1,
    contest_id: "contest", contest_login: "contest", contest_title: "Contest",
    created_at: "2026-10-11T10:00:00Z", updated_at: "2026-10-11T10:00:00Z",
  };

  it("renders frozen submissions with absent verdict and resources", () => {
    const view = renderView(<SubmissionsList submissions={[{...metadata, frozen: true}]} />);
    expect(view.container.textContent).toContain("Заморожено");
    expect(view.container.textContent).not.toContain("Неизвестно");
    expect(view.container.textContent).not.toContain(" ms");
    view.unmount();
    const details: SubmissionModel = {...metadata, frozen: true};
    const detailView = renderView(<SubmissionDetailsContent submission={details} />);
    expect(detailView.container.textContent).toContain("Результат скрыт до разморозки");
    expect(detailView.container.textContent).toContain("Исходный код скрыт");
    detailView.unmount();
  });

  it("preserves full own results and source", () => {
    const details: SubmissionModel = {...metadata, state: 200, score: 100, time_stat: 12, memory_stat: 256, submission: "print(42)"};
    const view = renderView(<SubmissionDetailsContent submission={details} />);
    expect(view.container.textContent).toContain("Принято");
    expect(view.container.textContent).toContain("12 ms");
    expect(view.container.textContent).toContain("print(42)");
    expect(view.container.textContent).not.toContain("Заморожено");
    view.unmount();
  });

  it("uses snapshots for frozen completion replay and reconnect recovery", async () => {
    let listener: Parameters<typeof submissionsWsManager.addListener>[0] | undefined;
    const add = spyOn(submissionsWsManager, "addListener").mockImplementation((params) => {
      listener = params; return 1;
    });
    const remove = spyOn(submissionsWsManager, "removeListener").mockImplementation(() => {});
    const wsUrl = spyOn(env, "getWebSocketUrl").mockReturnValue("ws://localhost/ws");
    const snapshot: ScoreboardResponseModel = {
      contest_id: "contest", contest_login: "contest", organization_login: "org", is_frozen: true, penalty_per_attempt: 20,
      problems: [{problem_id: "problem", title: "A+B", short_name: "A", ordinal: 1}],
      items: [{user_id: "user", username: "alice", problems_solved: 0, total_penalty: 0,
        problem_results: [{problem_id: "problem", solved: false, failed_attempts: 0, pending_attempts: 1}]}],
    };
    const fetch = spyOn(globalThis, "fetch").mockImplementation(Object.assign(async () => Response.json(snapshot), {preconnect: () => {}}));
    const view = renderWithProviders(<ContestMonitorTable contestId="contest" orgLogin="org" contestLogin="contest" initialScoreboard={snapshot} />);
    try {
      expect(listener!.url).toContain("contestId=contest");
      const event = {data: JSON.stringify({event_type: "submissions.completed", payload: {id: "sub", contest_id: "contest", frozen: true}})} as MessageEvent;
      await act(async () => {
 listener!.onMessage(event);
      });
      await act(async () => {
 listener!.onMessage(event);
      });
      expect(view.container.textContent).toContain("?1");
      expect(view.container.textContent).not.toContain("?2");
      const recovered = {...snapshot, is_frozen: false, items: [{...snapshot.items[0], problems_solved: 1,
        problem_results: [{problem_id: "problem", solved: true, failed_attempts: 0, pending_attempts: 0}]}]};
      fetch.mockImplementation(Object.assign(async () => Response.json(recovered), {preconnect: () => {}}));
      await act(async () => {
 listener!.onStatusChange({phase: "connected"});
      });
      await waitFor(() => expect(view.container.textContent).not.toContain("?1"));
      expect(view.container.textContent).toContain("+");
      await act(async () => {
        await listener!.onResyncRequired();
      });
      expect(String(fetch.mock.calls[0][0])).toContain("/organizations/org/contests/contest/scoreboard?unfrozen=false");
    } finally {
      view.unmount(); add.mockRestore(); remove.mockRestore(); wsUrl.mockRestore(); fetch.mockRestore();
    }
  });
});
