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
  selection: SubmissionsSelection, revisions: Map<string, number>, replay = false): LiveSubmissions | undefined => {
  if (!data || (!replay && event.sequence <= (data.since ?? 0)) || !metadataMatches(event.payload, selection)) {
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
  return {...data, since: Math.max(data.since ?? 0, event.sequence),
    submissions: data.submissions.map(s => s.id === payload.id ? {...s, progress} : s)};
};

export const applyScoreboardEvent = (data: LiveScoreboard | undefined, payload: MessageScoreboardUpdated,
  sequence: number): LiveScoreboard | undefined => {
  if (!data || sequence <= (data.since ?? 0)) {
    return data;
  }
  const next = payload.scoreboard;
  const items = payload.replace ? [...next.items]
    : [...data.items.filter(row => row.user_id !== payload.user_id), ...next.items];
  items.sort((a, b) => {
    const rank = b.problems_solved - a.problems_solved || a.total_penalty - b.total_penalty;
    if (rank) {
      return rank;
    }
    const lastA = a.last_accepted_at ? Date.parse(a.last_accepted_at) : Infinity;
    const lastB = b.last_accepted_at ? Date.parse(b.last_accepted_at) : Infinity;
    return lastA - lastB || Number(a.username > b.username) - Number(a.username < b.username);
  });
  return {...next, items, since: sequence};
};
