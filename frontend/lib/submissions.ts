import type {paths, SubmissionsListItemModel} from '@/contracts/core/v1/schema';

export interface SubmissionsSelection {
  orgLogin: string;
  contestLogin: string;
  contestId: string;
  userId?: string;
  problemId?: string;
  state?: number;
  language?: number;
  page: number;
  pageSize: number;
  sortOrder: 'asc' | 'desc';
}

export interface SubmissionWithProgress extends SubmissionsListItemModel {
  progress?: {phase: 'queued' | 'compiling' | 'testing'; testNumber?: number};
  isNew?: boolean;
}

export const SUBMISSIONS_PATH = '/organizations/{org_login}/contests/{contest_login}/submissions';

export const submissionsParams = ({orgLogin, contestLogin, contestId: _contestId, ...query}: SubmissionsSelection): {
  params: paths[typeof SUBMISSIONS_PATH]['get']['parameters'];
} => ({params: {path: {org_login: orgLogin, contest_login: contestLogin}, query}});
