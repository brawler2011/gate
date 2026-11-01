import {useContext} from 'react';

import {SubmissionsHighlights} from '@/contexts/SubmissionsProvider';
import {api} from '@/lib/api';
import {SUBMISSIONS_PATH, submissionsParams} from '@/lib/submissions';

import type {ListSubmissionsResponseModel} from '@/contracts/core/v1/schema';
import type {SubmissionsSelection, SubmissionWithProgress} from '@/lib/submissions';

export const useSubmissions = (selection: SubmissionsSelection): {
  data?: ListSubmissionsResponseModel; error: unknown;
  submissions: SubmissionWithProgress[]; highlightedIds: Set<string>;
} => {
  const query = api.useQuery('get', SUBMISSIONS_PATH, submissionsParams(selection), {meta: {state: selection.state}});
  const highlights = useContext(SubmissionsHighlights);
  const submissions: SubmissionWithProgress[] = (query.data?.submissions ?? [])
    .map(s => ({...s, isNew: highlights.get(s.id)}));
  return {data: query.data, error: query.error, submissions, highlightedIds: new Set(highlights.keys())};
};
