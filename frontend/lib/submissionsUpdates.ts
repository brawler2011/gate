import type {SubmissionsSelection, SubmissionWithProgress} from './submissions';
import type {ListSubmissionsResponseModel, ScoreboardResponseModel} from '@/contracts/core/v1/schema';
import type MessageScoreboardUpdated from '@/contracts/observer/v1/MessageScoreboardUpdated';
import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';

export type SubmissionEvent = Exclude<SubmissionsMessage, {event_type: 'scoreboard.updated'}>;
export type LiveSubmissions = Omit<ListSubmissionsResponseModel, 'submissions'> & {submissions: SubmissionWithProgress[]};
export type LiveScoreboard = ScoreboardResponseModel;

const metadataMatches = (row: SubmissionEvent['payload'], selection: SubmissionsSelection): boolean =>
  row.contest_id === selection.contestId && (!selection.userId || row.user_id === selection.userId)
  && (!selection.problemId || row.problem_id === selection.problemId)
  && (selection.language === undefined || row.language === selection.language);

// Durable events refetch lists; only transient progress is patched locally.
export const applySubmissionEvent = (data: LiveSubmissions | undefined, event: SubmissionEvent,
  selection: SubmissionsSelection, revisions: Map<string, number>): LiveSubmissions | undefined => {
  if (!data || !metadataMatches(event.payload, selection)) {
    return data;
  }
  const payload = event.payload;
  const existing = data.submissions.find(s => s.id === payload.id);
  if (!existing || payload.revision < Math.max(existing.revision, revisions.get(payload.id) ?? 0)) {
    return data;
  }
  let progress: SubmissionWithProgress['progress'];
  switch (event.event_type) {
    case 'submissions.queued': progress = {phase: 'queued'}; break;
    case 'submissions.compiling_started': progress = {phase: 'compiling'}; break;
    case 'submissions.testing_started': progress = {phase: 'testing'}; break;
    case 'submissions.test_started': progress = {phase: 'testing', testNumber: event.payload.number}; break;
    default: return data;
  }
  return {...data, submissions: data.submissions.map(s => s.id === payload.id ? {...s, progress} : s)};
};

// Updates arrive in commit order and are read after their change,
// so a row older than the snapshot is fixed by the next update.
export const applyScoreboardEvent = (data: LiveScoreboard | undefined,
  payload: MessageScoreboardUpdated): LiveScoreboard | undefined => {
  if (!data) {
    return data;
  }
  const next = payload.scoreboard;
  const items = payload.replace ? [...next.items]
    : [...data.items.filter(row => row.user_id !== payload.user_id), ...next.items];
  return {...next, items: rankScoreboard(items, next.ranking, next.tie_break)};
};

type ScoreboardItem = LiveScoreboard['items'][number];

// Mirrors the backend's rankItems: rows equal on every ranking key share a place.
const rankScoreboard = (items: ScoreboardItem[], ranking: LiveScoreboard['ranking'],
  tieBreak: LiveScoreboard['tie_break']): ScoreboardItem[] => {
  const time = (at?: string) => at ? Date.parse(at) : Infinity;
  // Infinity - Infinity is NaN, so two missing times compare equal.
  const byTime = (a?: string, b?: string) => Math.sign(time(a) - time(b)) || 0;
  const compare = (a: ScoreboardItem, b: ScoreboardItem): number => {
    if (ranking === 'points') {
      const score = (b.score ?? 0) - (a.score ?? 0);
      return score || (tieBreak === 'time' ? byTime(a.scored_at, b.scored_at) : 0);
    }
    return b.problems_solved - a.problems_solved || a.total_penalty - b.total_penalty
      || byTime(a.last_accepted_at, b.last_accepted_at);
  };
  const sorted = [...items].sort((a, b) =>
    compare(a, b) || Number(a.username > b.username) - Number(a.username < b.username));
  const ranked: ScoreboardItem[] = [];
  sorted.forEach((item, i) => {
    ranked.push({...item, rank: i > 0 && compare(sorted[i - 1], item) === 0 ? ranked[i - 1].rank : i + 1});
  });
  return ranked;
};
