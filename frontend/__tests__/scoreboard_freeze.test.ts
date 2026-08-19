import {describe, it, expect} from "bun:test";
import type {ScoreboardItemModel, ScoreboardProblemResultModel, ScoreboardProblemHeaderModel} from "../contracts/core/v1";

// Logic mirror extracted from ContestMonitorTable.tsx for empirical verification

export function computeCellDisplay(res?: ScoreboardProblemResultModel | null) {
  if (!res) {
    return {label: null, cssClass: null};
  }

  const hasPending = (res.pending_attempts || 0) > 0;
  const failed = res.failed_attempts || 0;
  const pending = res.pending_attempts || 0;

  if (res.solved) {
    let label = failed > 0 ? `+${failed}` : "+";
    if (hasPending) {
      label = `${label} ${pending}?`;
    }
    const cellClass = hasPending ? "cellFrozenPending" : "cellSolved";
    return {label, cssClass: cellClass};
  }

  if (hasPending) {
    let label = `?${pending}`;
    if (failed > 0) {
      label = `-${failed} ${pending}?`;
    }
    return {label, cssClass: "cellFrozenPending"};
  }

  if (failed > 0) {
    return {label: `-${failed}`, cssClass: "cellFailed"};
  }

  return {label: null, cssClass: null};
}

export function computeProblemSummary(
  problems: ScoreboardProblemHeaderModel[],
  items: ScoreboardItemModel[]
) {
  const solvedCounts: Record<string, number> = {};
  const attemptedCounts: Record<string, number> = {};

  for (const prob of problems) {
    solvedCounts[prob.problem_id] = 0;
    attemptedCounts[prob.problem_id] = 0;
  }

  for (const item of items) {
    for (const res of item.problem_results) {
      if (res.solved) {
        solvedCounts[res.problem_id] = (solvedCounts[res.problem_id] || 0) + 1;
        attemptedCounts[res.problem_id] = (attemptedCounts[res.problem_id] || 0) + 1;
      } else if ((res.failed_attempts || 0) > 0 || (res.pending_attempts || 0) > 0) {
        attemptedCounts[res.problem_id] = (attemptedCounts[res.problem_id] || 0) + 1;
      }
    }
  }

  return {solvedCounts, attemptedCounts};
}

export function handleWsSubmission(
  prevItems: ScoreboardItemModel[],
  payload: {user_id: string; problem_id: string; state: number; created_at?: string},
  isFrozen: boolean,
  showRealMonitor: boolean,
  startTime?: string | null,
  penaltyPerAttempt: number = 20
): ScoreboardItemModel[] {
  if (payload.state === 1 || payload.state === 101) {
    return prevItems;
  }

  if (isFrozen && !showRealMonitor) {
    const userId = payload.user_id;
    const problemId = payload.problem_id;

    const existingItemIndex = prevItems.findIndex((item) => item.user_id === userId);
    if (existingItemIndex === -1) {
      return prevItems;
    }

    const existingItem = prevItems[existingItemIndex];
    const pResults = [...existingItem.problem_results];
    const pIndex = pResults.findIndex((r) => r.problem_id === problemId);

    const pResult: ScoreboardProblemResultModel =
      pIndex !== -1
        ? {...pResults[pIndex]}
        : {
            problem_id: problemId,
            solved: false,
            failed_attempts: 0,
            pending_attempts: 0,
          };

    pResult.pending_attempts = (pResult.pending_attempts || 0) + 1;

    if (pIndex !== -1) {
      pResults[pIndex] = pResult;
    } else {
      pResults.push(pResult);
    }

    const updatedItem: ScoreboardItemModel = {
      ...existingItem,
      problem_results: pResults,
    };

    const newItems = [...prevItems];
    newItems[existingItemIndex] = updatedItem;
    return newItems;
  }

  // Live / Unfrozen branch
  const userId = payload.user_id;
  const problemId = payload.problem_id;
  const state = payload.state;
  const createdAtStr = payload.created_at || new Date().toISOString();

  const existingItemIndex = prevItems.findIndex((item) => item.user_id === userId);
  if (existingItemIndex === -1) {
    return prevItems;
  }

  const existingItem = prevItems[existingItemIndex];
  const pResults = [...existingItem.problem_results];
  const pIndex = pResults.findIndex((r) => r.problem_id === problemId);

  const pResult: ScoreboardProblemResultModel =
    pIndex !== -1
      ? {...pResults[pIndex]}
      : {
          problem_id: problemId,
          solved: false,
          failed_attempts: 0,
          pending_attempts: 0,
        };

  if (pResult.solved) {
    return prevItems;
  }

  if (state === 200) {
    pResult.solved = true;
    pResult.first_ac_time = createdAtStr;
    if (startTime) {
      const mins = Math.max(
        0,
        Math.floor((new Date(createdAtStr).getTime() - new Date(startTime).getTime()) / 60000)
      );
      pResult.time_minutes = mins;
    } else {
      pResult.time_minutes = 0;
    }
    pResult.penalty = pResult.failed_attempts * penaltyPerAttempt;
  } else if ([102, 103, 104, 105, 106].includes(state)) {
    pResult.failed_attempts += 1;
  } else {
    return prevItems;
  }

  if (pIndex !== -1) {
    pResults[pIndex] = pResult;
  } else {
    pResults.push(pResult);
  }

  let solvedCount = 0;
  let totalPenalty = 0;
  let lastAC: string | undefined = undefined;

  for (const r of pResults) {
    if (r.solved) {
      solvedCount += 1;
      totalPenalty += (r.time_minutes || 0) + r.failed_attempts * penaltyPerAttempt;
      if (r.first_ac_time) {
        if (!lastAC || new Date(r.first_ac_time).getTime() > new Date(lastAC).getTime()) {
          lastAC = r.first_ac_time;
        }
      }
    }
  }

  const updatedItem: ScoreboardItemModel = {
    ...existingItem,
    problems_solved: solvedCount,
    total_penalty: totalPenalty,
    last_accepted_at: lastAC,
    problem_results: pResults,
  };

  const newItems = [...prevItems];
  newItems[existingItemIndex] = updatedItem;

  newItems.sort((a, b) => {
    if (a.problems_solved !== b.problems_solved) {
      return b.problems_solved - a.problems_solved;
    }
    if (a.total_penalty !== b.total_penalty) {
      return a.total_penalty - b.total_penalty;
    }
    if (a.last_accepted_at && b.last_accepted_at) {
      const diff = new Date(a.last_accepted_at).getTime() - new Date(b.last_accepted_at).getTime();
      if (diff !== 0) {
        return diff;
      }
    } else if (a.last_accepted_at) {
      return -1;
    } else if (b.last_accepted_at) {
      return 1;
    }
    return a.username.localeCompare(b.username);
  });

  return newItems;
}

describe("Frontend Scoreboard Freeze Empirical Tests", () => {
  describe("1. Cell Display Text & Styling Matrix", () => {
    it("renders ?k for no pre-freeze attempts + k frozen attempts", () => {
      for (let k = 1; k <= 10; k++) {
        const res: ScoreboardProblemResultModel = {
          problem_id: "p1",
          solved: false,
          failed_attempts: 0,
          pending_attempts: k,
        };
        const cell = computeCellDisplay(res);
        expect(cell.label).toBe(`?${k}`);
        expect(cell.cssClass).toBe("cellFrozenPending");
      }
    });

    it("renders -k p? for k failed pre-freeze attempts + p frozen attempts", () => {
      for (let k = 1; k <= 5; k++) {
        for (let p = 1; p <= 5; p++) {
          const res: ScoreboardProblemResultModel = {
            problem_id: "p1",
            solved: false,
            failed_attempts: k,
            pending_attempts: p,
          };
          const cell = computeCellDisplay(res);
          expect(cell.label).toBe(`-${k} ${p}?`);
          expect(cell.cssClass).toBe("cellFrozenPending");
        }
      }
    });

    it("renders + p? for solved pre-freeze (0 fails) + p frozen attempts", () => {
      for (let p = 1; p <= 5; p++) {
        const res: ScoreboardProblemResultModel = {
          problem_id: "p1",
          solved: true,
          failed_attempts: 0,
          pending_attempts: p,
        };
        const cell = computeCellDisplay(res);
        expect(cell.label).toBe(`+ ${p}?`);
        expect(cell.cssClass).toBe("cellFrozenPending");
      }
    });

    it("renders +k p? for solved pre-freeze (k fails) + p frozen attempts", () => {
      for (let k = 1; k <= 5; k++) {
        for (let p = 1; p <= 5; p++) {
          const res: ScoreboardProblemResultModel = {
            problem_id: "p1",
            solved: true,
            failed_attempts: k,
            pending_attempts: p,
          };
          const cell = computeCellDisplay(res);
          expect(cell.label).toBe(`+${k} ${p}?`);
          expect(cell.cssClass).toBe("cellFrozenPending");
        }
      }
    });

    it("renders standard pre-freeze labels when pending_attempts is 0 or undefined", () => {
      // Solved, 0 fails
      expect(
        computeCellDisplay({
          problem_id: "p1",
          solved: true,
          failed_attempts: 0,
          pending_attempts: 0,
        })
      ).toEqual({label: "+", cssClass: "cellSolved"});

      // Solved, 3 fails
      expect(
        computeCellDisplay({
          problem_id: "p1",
          solved: true,
          failed_attempts: 3,
          pending_attempts: 0,
        })
      ).toEqual({label: "+3", cssClass: "cellSolved"});

      // Failed, 2 fails
      expect(
        computeCellDisplay({
          problem_id: "p1",
          solved: false,
          failed_attempts: 2,
          pending_attempts: 0,
        })
      ).toEqual({label: "-2", cssClass: "cellFailed"});

      // Not attempted
      expect(
        computeCellDisplay({
          problem_id: "p1",
          solved: false,
          failed_attempts: 0,
          pending_attempts: 0,
        })
      ).toEqual({label: null, cssClass: null});

      // Null / undefined result
      expect(computeCellDisplay(null)).toEqual({label: null, cssClass: null});
    });
  });

  describe("2. Problem Summary Calculations (Сдали and Пытались)", () => {
    const problems: ScoreboardProblemHeaderModel[] = [
      {problem_id: "prob-A", title: "Problem A", ordinal: 1, short_name: "A"},
      {problem_id: "prob-B", title: "Problem B", ordinal: 2, short_name: "B"},
      {problem_id: "prob-C", title: "Problem C", ordinal: 3, short_name: "C"},
    ];

    it("correctly calculates solved and attempted counts with frozen attempts", () => {
      const items: ScoreboardItemModel[] = [
        {
          user_id: "u1",
          username: "Alice",
          problems_solved: 1,
          total_penalty: 10,
          problem_results: [
            {problem_id: "prob-A", solved: true, failed_attempts: 0, pending_attempts: 0}, // Solved
            {problem_id: "prob-B", solved: false, failed_attempts: 2, pending_attempts: 1}, // Attempted (pre-freeze fails + frozen)
            {problem_id: "prob-C", solved: false, failed_attempts: 0, pending_attempts: 3}, // Attempted (frozen only)
          ],
        },
        {
          user_id: "u2",
          username: "Bob",
          problems_solved: 0,
          total_penalty: 0,
          problem_results: [
            {problem_id: "prob-A", solved: false, failed_attempts: 1, pending_attempts: 0}, // Attempted (failed)
            {problem_id: "prob-B", solved: false, failed_attempts: 0, pending_attempts: 0}, // Not attempted
            {problem_id: "prob-C", solved: false, failed_attempts: 0, pending_attempts: 1}, // Attempted (frozen only)
          ],
        },
        {
          user_id: "u3",
          username: "Charlie",
          problems_solved: 1,
          total_penalty: 50,
          problem_results: [
            {problem_id: "prob-A", solved: false, failed_attempts: 0, pending_attempts: 0}, // Not attempted
            {problem_id: "prob-B", solved: true, failed_attempts: 1, pending_attempts: 2}, // Solved + frozen
            {problem_id: "prob-C", solved: false, failed_attempts: 0, pending_attempts: 0}, // Not attempted
          ],
        },
      ];

      const summary = computeProblemSummary(problems, items);

      // Prob A: Alice solved (+), Bob failed (-1), Charlie nothing
      // -> Solved = 1 (Alice), Attempted = 2 (Alice, Bob)
      expect(summary.solvedCounts["prob-A"]).toBe(1);
      expect(summary.attemptedCounts["prob-A"]).toBe(2);

      // Prob B: Alice failed + frozen (-2 1?), Bob nothing, Charlie solved + frozen (+1 2?)
      // -> Solved = 1 (Charlie), Attempted = 2 (Alice, Charlie)
      expect(summary.solvedCounts["prob-B"]).toBe(1);
      expect(summary.attemptedCounts["prob-B"]).toBe(2);

      // Prob C: Alice frozen only (?3), Bob frozen only (?1), Charlie nothing
      // -> Solved = 0, Attempted = 2 (Alice, Bob)
      expect(summary.solvedCounts["prob-C"]).toBe(0);
      expect(summary.attemptedCounts["prob-C"]).toBe(2);
    });

    it("does not count frozen submissions towards 'Сдали'", () => {
      const items: ScoreboardItemModel[] = [
        {
          user_id: "u1",
          username: "Dave",
          problems_solved: 0,
          total_penalty: 0,
          problem_results: [
            {problem_id: "prob-A", solved: false, failed_attempts: 0, pending_attempts: 5},
          ],
        },
      ];

      const summary = computeProblemSummary(problems, items);
      expect(summary.solvedCounts["prob-A"]).toBe(0);
      expect(summary.attemptedCounts["prob-A"]).toBe(1);
    });
  });

  describe("3. Real-time WebSocket Behavior during Scoreboard Freeze", () => {
    const initialItems: ScoreboardItemModel[] = [
      {
        user_id: "u1",
        username: "Alice",
        problems_solved: 2,
        total_penalty: 100,
        problem_results: [
          {problem_id: "prob-A", solved: true, failed_attempts: 0, pending_attempts: 0, time_minutes: 30},
          {problem_id: "prob-B", solved: true, failed_attempts: 1, pending_attempts: 0, time_minutes: 50},
          {problem_id: "prob-C", solved: false, failed_attempts: 2, pending_attempts: 0},
        ],
      },
      {
        user_id: "u2",
        username: "Bob",
        problems_solved: 1,
        total_penalty: 40,
        problem_results: [
          {problem_id: "prob-A", solved: true, failed_attempts: 0, pending_attempts: 0, time_minutes: 40},
          {problem_id: "prob-B", solved: false, failed_attempts: 1, pending_attempts: 0},
        ],
      },
      {
        user_id: "u3",
        username: "Charlie",
        problems_solved: 0,
        total_penalty: 0,
        problem_results: [],
      },
    ];

    it("increments pending_attempts during freeze without altering score, penalty or rank for AC verdict", () => {
      // Bob submits prob-B and gets AC (state 200) during freeze
      const updated = handleWsSubmission(
        initialItems,
        {
          user_id: "u2",
          problem_id: "prob-B",
          state: 200, // Accepted!
          created_at: "2026-08-19T18:00:00Z",
        },
        true, // isFrozen
        false // showRealMonitor
      );

      // Order must remain Alice (1st), Bob (2nd), Charlie (3rd)
      expect(updated[0].user_id).toBe("u1");
      expect(updated[1].user_id).toBe("u2");
      expect(updated[2].user_id).toBe("u3");

      // Bob's score and penalty must be unchanged
      expect(updated[1].problems_solved).toBe(1);
      expect(updated[1].total_penalty).toBe(40);

      // Bob's prob-B result must have pending_attempts = 1, solved still false
      const bobProbB = updated[1].problem_results.find((r) => r.problem_id === "prob-B");
      expect(bobProbB).toBeDefined();
      expect(bobProbB!.solved).toBe(false);
      expect(bobProbB!.failed_attempts).toBe(1);
      expect(bobProbB!.pending_attempts).toBe(1);

      // Cell display for Bob prob-B must be "-1 1?"
      const cell = computeCellDisplay(bobProbB);
      expect(cell.label).toBe("-1 1?");
      expect(cell.cssClass).toBe("cellFrozenPending");
    });

    it("increments pending_attempts for a new problem never attempted before by a participant", () => {
      // Charlie submits prob-C during freeze (state 102 WA)
      const updated = handleWsSubmission(
        initialItems,
        {
          user_id: "u3",
          problem_id: "prob-C",
          state: 102, // WA
          created_at: "2026-08-19T18:05:00Z",
        },
        true,
        false
      );

      expect(updated[2].user_id).toBe("u3");
      expect(updated[2].problems_solved).toBe(0);
      expect(updated[2].total_penalty).toBe(0);

      const charlieProbC = updated[2].problem_results.find((r) => r.problem_id === "prob-C");
      expect(charlieProbC).toBeDefined();
      expect(charlieProbC!.solved).toBe(false);
      expect(charlieProbC!.failed_attempts).toBe(0);
      expect(charlieProbC!.pending_attempts).toBe(1);

      // Cell display for Charlie prob-C must be "?1"
      const cell = computeCellDisplay(charlieProbC);
      expect(cell.label).toBe("?1");
      expect(cell.cssClass).toBe("cellFrozenPending");
    });

    it("accumulates multiple frozen submissions on the same problem", () => {
      let current = initialItems;

      // Charlie submits 3 times to prob-C during freeze
      for (let i = 1; i <= 3; i++) {
        current = handleWsSubmission(
          current,
          {
            user_id: "u3",
            problem_id: "prob-C",
            state: i === 3 ? 200 : 102,
            created_at: "2026-08-19T18:10:00Z",
          },
          true,
          false
        );
      }

      const charlieProbC = current[2].problem_results.find((r) => r.problem_id === "prob-C");
      expect(charlieProbC!.pending_attempts).toBe(3);
      expect(charlieProbC!.solved).toBe(false);

      const cell = computeCellDisplay(charlieProbC);
      expect(cell.label).toBe("?3");
      expect(cell.cssClass).toBe("cellFrozenPending");
    });

    it("submitting during freeze on an already solved problem increments pending_attempts without breaking solved status", () => {
      // Alice submits prob-A again during freeze
      const updated = handleWsSubmission(
        initialItems,
        {
          user_id: "u1",
          problem_id: "prob-A",
          state: 200,
        },
        true,
        false
      );

      const aliceProbA = updated[0].problem_results.find((r) => r.problem_id === "prob-A");
      expect(aliceProbA!.solved).toBe(true);
      expect(aliceProbA!.pending_attempts).toBe(1);

      const cell = computeCellDisplay(aliceProbA);
      expect(cell.label).toBe("+ 1?");
      expect(cell.cssClass).toBe("cellFrozenPending");
    });

    it("when showRealMonitor is true (organizer live view), updates score and re-sorts standings", () => {
      // Bob submits prob-B and gets AC (state 200) with showRealMonitor=true
      const updated = handleWsSubmission(
        initialItems,
        {
          user_id: "u2",
          problem_id: "prob-B",
          state: 200,
          created_at: "2026-08-19T17:50:00Z",
        },
        true, // isFrozen=true
        true, // showRealMonitor=true (organizer toggled live view!)
        "2026-08-19T17:00:00Z", // startTime (50 min into contest)
        20 // penaltyPerAttempt
      );

      const bob = updated.find((u) => u.user_id === "u2");
      expect(bob).toBeDefined();
      expect(bob!.problems_solved).toBe(2);
      // Bob penalty = prob-A (40) + prob-B (50 min + 1 fail * 20 = 70) = 110
      expect(bob!.total_penalty).toBe(110);

      // Standings: Alice (2 solved, 100 pen), Bob (2 solved, 110 pen)
      expect(updated[0].user_id).toBe("u1"); // Alice 100 pen
      expect(updated[1].user_id).toBe("u2"); // Bob 110 pen
    });

    it("ignores in-progress evaluation states (state 1 and 101)", () => {
      const updated1 = handleWsSubmission(
        initialItems,
        {
          user_id: "u1",
          problem_id: "prob-C",
          state: 1, // submitted
        },
        true,
        false
      );
      expect(updated1).toBe(initialItems);

      const updated2 = handleWsSubmission(
        initialItems,
        {
          user_id: "u1",
          problem_id: "prob-C",
          state: 101, // compiling
        },
        true,
        false
      );
      expect(updated2).toBe(initialItems);
    });
  });

  describe("4. Stress & Edge Cases", () => {
    it("handles empty items array in problem summary without throwing", () => {
      const problems: ScoreboardProblemHeaderModel[] = [
        {problem_id: "p1", title: "P1", ordinal: 1, short_name: "A"},
      ];
      const summary = computeProblemSummary(problems, []);
      expect(summary.solvedCounts["p1"]).toBe(0);
      expect(summary.attemptedCounts["p1"]).toBe(0);
    });

    it("handles negative or invalid pending_attempts safely", () => {
      const res: ScoreboardProblemResultModel = {
        problem_id: "p1",
        solved: false,
        failed_attempts: 0,
        pending_attempts: -1,
      };
      const cell = computeCellDisplay(res);
      // (pending_attempts || 0) > 0 is false -> not pending
      expect(cell.label).toBe(null);
      expect(cell.cssClass).toBe(null);
    });

    it("validates SettingsSection freeze_duration_minutes logic", () => {
      const validator = (value: unknown) =>
        value !== "" && value !== undefined && value !== null && Number(value) < 0
          ? "Длительность заморозки не может быть отрицательной"
          : null;

      expect(validator("")).toBe(null);
      expect(validator(null)).toBe(null);
      expect(validator(undefined)).toBe(null);
      expect(validator(0)).toBe(null);
      expect(validator("0")).toBe(null);
      expect(validator(60)).toBe(null);
      expect(validator("60")).toBe(null);
      expect(validator(-1)).toBe("Длительность заморозки не может быть отрицательной");
      expect(validator("-5")).toBe("Длительность заморозки не может быть отрицательной");
    });
  });
});
