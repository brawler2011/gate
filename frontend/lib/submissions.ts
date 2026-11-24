import type {paths, SubmissionsListItemModel} from '@/contracts/core/v1/schema';

export interface SubmissionsSelection {
  orgLogin: string;
  contestLogin: string;
  contestId: string;
  userId?: string;
  problemId?: string;
  state?: number;
  language?: string;
  page: number;
  pageSize: number;
  sortOrder: 'asc' | 'desc';
}

export interface SubmissionWithProgress extends SubmissionsListItemModel {
  progress?: {phase: 'queued' | 'compiling' | 'testing'; testNumber?: number};
  isNew?: boolean;
}

export const SUBMISSIONS_PATH = '/contests/{contest_id}/submissions';

export const submissionsParams = (
  {orgLogin: _orgLogin, contestLogin: _contestLogin, contestId, ...query}: SubmissionsSelection,
): {
  params: paths[typeof SUBMISSIONS_PATH]['get']['parameters'];
} => ({params: {path: {contest_id: contestId}, query}});
