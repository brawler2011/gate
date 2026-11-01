import './setup-dom';

import {afterEach, expect, it, spyOn} from 'bun:test';

import {SessionProvider} from '@/contexts/SessionContext';
import {env} from '@/lib/env';
import * as submissionsConnection from '@/lib/submissionsConnection';
import Page from '@/pages/contests/task/TaskPage';

import {act, cleanup, renderWithProviders, setupDOMEnvironment, waitFor} from './test-utils';

const user = {id: 'user', username: 'alice', role: 'user', createdAt: '2026-10-01', updatedAt: '2026-10-01'};
const problem = {
  problem_id: 'problem', title: 'Snapshot problem', position: 1, time_limit: 1000, memory_limit: 256,
  legend_html: '', input_format_html: '', output_format_html: '', notes_html: '', scoring_html: '',
  created_at: '2026-10-01', updated_at: '2026-10-01', samples: [],
};

afterEach(cleanup);

it.each(['success', 'failure', 'guest'])('handles the initial submissions snapshot: %s', async (outcome) => {
  const closeDOM = setupDOMEnvironment();
  const wsUrl = spyOn(env, 'getWebSocketUrl').mockReturnValue('ws://localhost/ws');
  const listener = spyOn(submissionsConnection, 'connectSubmissions').mockReturnValue(() => {});
  let finishSubmissions!: (response: Response) => void;
  const pendingSubmissions = new Promise<Response>((resolve) => {
    finishSubmissions = resolve;
  });
  let finishProblem!: (response: Response) => void;
  const pendingProblem = new Promise<Response>((resolve) => {
    finishProblem = resolve;
  });
  const fetch = spyOn(globalThis, 'fetch').mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost');
    if (url.pathname === '/api/users/me') {
      return Response.json({user: null});
    }
    if (url.pathname.endsWith('/my-role')) {
      return Response.json({role: 'participant'});
    }
    if (url.pathname.endsWith('/problems/problem')) {
      return pendingProblem;
    }
    if (url.pathname.endsWith('/submissions')) {
      return pendingSubmissions;
    }
    if (url.pathname === '/api/organizations/school/contests/round') {
      return Response.json({contest: {id: 'contest', login: 'round', organization_login: 'school',
        visibility: 'public'}, problems: [problem]});
    }
    throw new Error(`Unexpected API request ${url.pathname}`);
  }, {preconnect: () => {}}));
  try {
    const page = <SessionProvider initialUser={outcome === 'guest' ? null : user}><Page /></SessionProvider>;
    const view = renderWithProviders(page, {route: '/school/round/A',
      routes: [{path: '/:slug/:contestLogin/:letter', element: page}]});
    await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).endsWith('/problems/problem'))).toBe(true));
    await act(async () => {
      finishProblem(Response.json({problem}));
      await pendingProblem;
    });
    if (outcome === 'guest') {
      await waitFor(() => expect(view.queryByRole('heading', {name: 'A. Snapshot problem'})).toBeTruthy());
      expect(fetch.mock.calls.some(([url]) => String(url).includes('/submissions'))).toBe(false);
    } else {
      await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url).includes('/submissions'))).toBe(true));
      expect(Boolean(view.queryByRole('heading', {name: 'A. Snapshot problem'}))).toBe(false);
      expect(listener).not.toHaveBeenCalled();
      await act(async () => {
        finishSubmissions(outcome === 'success'
          ? Response.json({submissions: [{id: 'submission', state: 2, score: 73, created_at: '2026-10-01T00:00:00Z'}], since: 42})
          : Response.json({message: 'Unavailable'}, {status: 500}));
        await pendingSubmissions;
      });
      await waitFor(() => expect(view.queryByRole('heading', {name: 'A. Snapshot problem'})).toBeTruthy());
      if (outcome === 'success') {
        await waitFor(() => expect(view.queryByRole('cell', {name: '73'})).toBeTruthy());
        expect(listener.mock.calls[0]?.[0].url()).toContain('since=42');
      }
    }
  } finally {
    closeDOM();
    fetch.mockRestore();
    wsUrl.mockRestore();
    listener.mockRestore();
  }
});
