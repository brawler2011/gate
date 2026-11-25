import {describe, expect, it} from 'bun:test';

import ContestRanking from '@/contracts/observer/v1/ContestRanking';
import ContestTieBreak from '@/contracts/observer/v1/ContestTieBreak';
import {applyScoreboardEvent} from '@/lib/submissionsUpdates';

import type ScoreboardItemModel from '@/contracts/observer/v1/ScoreboardItemModel';
import type ScoreboardResponseModel from '@/contracts/observer/v1/ScoreboardResponseModel';

const row = (user_id: string, problems_solved = 0, total_penalty = 0,
  last_accepted_at?: string): ScoreboardItemModel => ({
  user_id, username: user_id, problems_solved, total_penalty, last_accepted_at, rank: 1, problem_results: [],
});
const scored = (user_id: string, score: number, scored_at: string): ScoreboardItemModel =>
  ({...row(user_id), score, scored_at});
const scoreboard = (...items: ScoreboardItemModel[]): ScoreboardResponseModel => ({
  contest_id: 'contest', contest_login: 'round', organization_login: 'org', penalty_per_attempt: 20,
  ranking: ContestRanking.ICPC, tie_break: ContestTieBreak.SHARED, is_frozen: false, problems: [], items,
});

describe('authoritative scoreboard updates', () => {
  it('replaces a participant row and sorts by solved, penalty, last acceptance and username', () => {
    const snapshot = scoreboard(row('alice'), row('bob', 1, 40, '2026-01-01T00:10:00Z'), row('zoe', 1, 20),
      row('carl', 1, 40, '2026-01-01T00:15:00Z'), row('dave', 1, 40, '2026-01-01T00:15:00Z'));
    const update = {user_id: 'alice', unfrozen: false, replace: false,
      scoreboard: scoreboard(row('alice', 2, 100))};
    const next = applyScoreboardEvent(snapshot, update)!;
    expect(next.items.map(s => s.user_id)).toEqual(['alice', 'zoe', 'bob', 'carl', 'dave']);
    expect(applyScoreboardEvent(next, update)?.items).toEqual(next.items);
  });

  it('ranks merged points rows by total, sharing places unless ties go by time', () => {
    const points = (tie_break: ScoreboardResponseModel['tie_break'], ...items: ScoreboardItemModel[]) =>
      ({...scoreboard(...items), ranking: ContestRanking.POINTS, tie_break});
    const places = (tie_break: ScoreboardResponseModel['tie_break']) => applyScoreboardEvent(
      points(tie_break, scored('carol', 120, '2026-01-01T00:10:00Z'), scored('bob', 50, '2026-01-01T00:05:00Z')),
      {user_id: 'alice', unfrozen: false, replace: false,
        scoreboard: points(tie_break, {...scored('alice', 120, '2026-01-01T00:20:00Z'), rank: 0})},
    )!.items.map(s => [s.user_id, s.rank]);
    expect(places(ContestTieBreak.SHARED)).toEqual([['alice', 1], ['carol', 1], ['bob', 3]]);
    expect(places(ContestTieBreak.TIME)).toEqual([['carol', 1], ['alice', 2], ['bob', 3]]);
  });

  it('breaks time ties within a millisecond as the backend does', () => {
    const snapshot = {...scoreboard(scored('bob', 10, '2026-01-01T00:00:00.1239Z')), ranking: ContestRanking.POINTS,
      tie_break: ContestTieBreak.TIME};
    const update = {user_id: 'zed', unfrozen: false, replace: false,
      scoreboard: {...snapshot, items: [scored('zed', 10, '2026-01-01T00:00:00.1231Z')]}};
    expect(applyScoreboardEvent(snapshot, update)!.items.map(s => [s.user_id, s.rank])).toEqual([['zed', 1], ['bob', 2]]);
  });

  it('replaces all rows at freeze transitions and removes participants excluded by the server', () => {
    const snapshot = scoreboard(row('alice', 1), row('bob'));
    const frozen = {...scoreboard(row('bob')), is_frozen: true};
    const next = applyScoreboardEvent(snapshot, {user_id: 'alice', unfrozen: false, replace: true, scoreboard: frozen})!;
    expect(next.items).toEqual(frozen.items);
    expect(next.is_frozen).toBe(true);
    expect(applyScoreboardEvent(next, {user_id: 'bob', unfrozen: false, replace: false, scoreboard: scoreboard()})?.items).toEqual([]);
  });
});
