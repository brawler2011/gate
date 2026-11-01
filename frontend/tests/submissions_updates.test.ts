import {describe, expect, it} from 'bun:test';

import {applyScoreboardEvent} from '@/lib/submissionsUpdates';

import type ScoreboardItemModel from '@/contracts/observer/v1/ScoreboardItemModel';
import type ScoreboardResponseModel from '@/contracts/observer/v1/ScoreboardResponseModel';

const row = (user_id: string, problems_solved = 0, total_penalty = 0,
  last_accepted_at?: string): ScoreboardItemModel => ({
  user_id, username: user_id, problems_solved, total_penalty, last_accepted_at, problem_results: [],
});
const scoreboard = (...items: ScoreboardItemModel[]): ScoreboardResponseModel => ({
  contest_id: 'contest', contest_login: 'round', organization_login: 'org', penalty_per_attempt: 20,
  is_frozen: false, problems: [], items,
});

describe('authoritative scoreboard updates', () => {
  it('replaces a participant row and sorts by solved, penalty, last acceptance and username', () => {
    const snapshot = scoreboard(row('alice'), row('bob', 1, 40, '2026-01-01T00:10:00Z'), row('zoe', 1, 20),
      row('carl', 1, 40, '2026-01-01T00:15:00Z'), row('dave', 1, 40, '2026-01-01T00:15:00Z'));
    const update = {user_id: 'alice', unfrozen: false, replace: false,
      scoreboard: scoreboard(row('alice', 2, 100))};
    const next = applyScoreboardEvent(snapshot, update, 8)!;
    expect(next.items.map(s => s.user_id)).toEqual(['alice', 'zoe', 'bob', 'carl', 'dave']);
    expect(applyScoreboardEvent(next, update, 8)).toBe(next);
    expect(applyScoreboardEvent({...snapshot, since: 10}, update, 9)?.items).toEqual(snapshot.items);
  });

  it('replaces all rows at freeze transitions and removes participants excluded by the server', () => {
    const snapshot = scoreboard(row('alice', 1), row('bob'));
    const frozen = {...scoreboard(row('bob')), is_frozen: true};
    const next = applyScoreboardEvent(snapshot, {user_id: 'alice', unfrozen: false, replace: true, scoreboard: frozen}, 8)!;
    expect(next.items).toEqual(frozen.items);
    expect(next.is_frozen).toBe(true);
    expect(applyScoreboardEvent(next, {user_id: 'bob', unfrozen: false, replace: false, scoreboard: scoreboard()}, 9)?.items).toEqual([]);
  });
});
