import type {SubmissionsSelection, SubmissionWithProgress} from './submissions';
import type {ListSubmissionsResponseModel, ScoreboardResponseModel, SubmissionsListItemModel} from '@/contracts/core/v1/schema';
import type MessageScoreboardUpdated from '@/contracts/observer/v1/MessageScoreboardUpdated';
import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';

type SubmissionEvent = Exclude<SubmissionsMessage, {event_type: 'scoreboard.updated'}>;
export type LiveSubmissions = Omit<ListSubmissionsResponseModel, 'submissions'> & {submissions: SubmissionWithProgress[]};
export type LiveScoreboard = ScoreboardResponseModel;

const metadataMatches = (row: SubmissionEvent['payload'], selection: SubmissionsSelection): boolean =>
  row.contest_id === selection.contestId && (!selection.userId || row.user_id === selection.userId)
  && (!selection.problemId || row.problem_id === selection.problemId)
  && (selection.language === undefined || row.language === selection.language);

const completeRow = (row: Partial<SubmissionsListItemModel>): row is SubmissionsListItemModel =>
  typeof row.revision === 'number' && typeof row.id === 'string' && typeof row.user_id === 'string' && typeof row.username === 'string'
  && typeof row.contest_id === 'string' && typeof row.contest_login === 'string' && typeof row.contest_title === 'string'
  && typeof row.problem_id === 'string' && typeof row.problem_title === 'string' && typeof row.position === 'number'
  && typeof row.language === 'number' && typeof row.created_at === 'string' && typeof row.updated_at === 'string';

export const applySubmissionEvent = (data: LiveSubmissions | undefined, event: SubmissionEvent,
  selection: SubmissionsSelection): LiveSubmissions | undefined => {
  if (!data || event.sequence <= (data.since ?? 0)) {
    return data;
  }
  const payload = event.payload;
  if (!metadataMatches(payload, selection)) {
    return {...data, since: event.sequence};
  }
  const existing = data.submissions.find(s => s.id === payload.id);
  let progress: SubmissionWithProgress['progress'];
  switch (event.event_type) {
    case 'submissions.queued': progress = {phase: 'queued'}; break;
    case 'submissions.compiling_started': progress = {phase: 'compiling'}; break;
    case 'submissions.testing_started': progress = {phase: 'testing'}; break;
    case 'submissions.test_started': progress = {phase: 'testing', testNumber: event.payload.number}; break;
    default: break;
  }
  if (progress) {
    if (existing && event.payload.revision < existing.revision) {
      return {...data, since: event.sequence};
    }
    return {...data, since: event.sequence,
      submissions: data.submissions.map(s => s.id === payload.id ? {...s, progress} : s)};
  }
  if (event.event_type !== 'submissions.created' && event.event_type !== 'submissions.completed') {
    return data;
  }
  // Keep the snapshot watermark: a bulk update can emit several events with the same revision.
  if (typeof event.payload.revision !== 'number' || event.payload.revision <= data.revision) {
    return {...data, since: event.sequence};
  }
  const live = selection.page === 1 && selection.sortOrder === 'desc';
  const p = event.payload;
  const isNew = event.event_type === 'submissions.created' && event.payload.is_new;
  const before = !isNew && (selection.state === undefined || p.previous_state === selection.state);
  const after = selection.state === undefined || p.state === selection.state;
  const row = {...existing, id: p.id, revision: p.revision,
    user_id: p.user_id ?? existing?.user_id, username: p.username ?? existing?.username,
    contest_id: p.contest_id ?? existing?.contest_id, contest_login: selection.contestLogin,
    contest_title: p.contest_title ?? existing?.contest_title,
    problem_id: p.problem_id ?? existing?.problem_id, problem_title: p.problem_title ?? existing?.problem_title,
    position: p.position ?? existing?.position, language: p.language ?? existing?.language,
    created_at: p.created_at ?? existing?.created_at, updated_at: p.updated_at ?? existing?.updated_at,
    frozen: p.frozen, progress: undefined, state: p.state,
    score: 'score' in p ? p.score : undefined, penalty: 'penalty' in p ? p.penalty : undefined,
    time_stat: 'time_stat' in p ? p.time_stat : undefined, memory_stat: 'memory_stat' in p ? p.memory_stat : undefined,
    failed_test: 'failed_test' in p ? p.failed_test : undefined};
  if (after && !completeRow(row)) {
    return data;
  }
  const replacement = after && completeRow(row) ? [row] : [];
  const stale = existing !== undefined && existing.revision >= p.revision;
  const rows = data.submissions.flatMap(s => s.id === p.id && !stale ? replacement : [s]);
  if (!existing && live) {
    rows.unshift(...replacement);
  }
  if (live) {
    rows.sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
  }
  const total_count = data.total_count + Number(after) - Number(before);
  return {...data, since: event.sequence, total_count, submissions: rows.slice(0, selection.pageSize),
    pagination: {...data.pagination, total: Math.ceil(Math.max(0, total_count) / selection.pageSize)}};
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
