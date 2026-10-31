import './setup-dom';

import {afterAll, afterEach, beforeAll, describe, expect, it, spyOn} from 'bun:test';
import {useEffect} from 'react';
import {Link, matchRoutes} from 'react-router-dom';

import {RouteBoundary} from '@/components/shared/RouteBoundary';

import {cleanup, fireEvent, renderWithProviders, setupDOMEnvironment, waitFor} from './test-utils';
import {App, appRoutes} from '../../src/App';

import type {ReactNode} from 'react';

describe('SPA route tree', () => {
  let closeDOM: () => void;
  beforeAll(() => {
    closeDOM = setupDOMEnvironment();
  });
  afterEach(cleanup);
  afterAll(() => {
    closeDOM();
  });

  it('matches contest parameters and gives static sections priority over task letters', () => {
    const task = matchRoutes(appRoutes, '/school/round/A')!;
    expect(task.at(-1)!.params).toEqual({slug: 'school', contestLogin: 'round', letter: 'A'});
    expect(task.at(-1)!.route.path).toBe(':letter');
    for (const section of ['monitor', 'settings', 'submissions', 'mysubmissions', 'submit', 'drafts', 'messages']) {
      const matches = matchRoutes(appRoutes, `/school/round/${section}`)!;
      expect(matches.at(-1)!.route.path).toBe(section);
      expect(matches.at(-2)!.route).toBe(task.at(-2)!.route);
    }
    const editor = matchRoutes(appRoutes, '/school/problems/problem/tests')!;
    expect(editor.at(-1)!.route.path).toBe('tests');
    expect(editor.at(-1)!.params).toEqual({slug: 'school', problem_id: 'problem'});
    expect(matchRoutes(appRoutes, '/admin/users/alice')!.at(-1)!.route.path).toBe('users/:username');
  });

  it('routes /@alice to the public profile and blocks the retired internal profile URL', async () => {
    const fetch = spyOn(globalThis, 'fetch').mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname === '/api/users/me') {
        return Response.json({message: 'Unauthorized'}, {status: 401});
      }
      if (url.pathname === '/api/users/alice') {
        return Response.json({user: {id: 'alice', username: 'alice', role: 'user',
          createdAt: '2026-10-01', updatedAt: '2026-10-01'}});
      }
      if (url.pathname === '/api/users/alice/contests') {
        return Response.json({contests: [], pagination: {page: 1, total: 0}});
      }
      throw new Error(`Unexpected API request ${url.pathname}`);
    }, {preconnect: () => {}}));
    try {
      const view = renderWithProviders(<App />, {route: '/@alice'});
      await waitFor(() => expect(fetch.mock.calls.some(([url]) => String(url) === '/api/users/alice')).toBe(true));
      await waitFor(() => expect(view.queryByRole('heading', {name: '@alice'})).toBeTruthy());
      view.unmount();
      fetch.mockClear();
      const blocked = renderWithProviders(<App />, {route: '/user/alice'});
      expect(blocked.getByRole('heading', {name: 'Страница не найдена'})).toBeTruthy();
      expect(fetch).not.toHaveBeenCalled();
    } finally {
      fetch.mockRestore();
    }
  });

  it('keeps the shared boundary subtree mounted during navigation', () => {
    let mounts = 0;
    const Layout = (): ReactNode => {
      useEffect(() => {
        mounts++;
      }, []);
      return <Link to="/school/round/monitor">Монитор</Link>;
    };
    const view = renderWithProviders(<RouteBoundary><Layout /></RouteBoundary>, {route: '/school/round/A'});
    fireEvent.click(view.getByRole('link', {name: 'Монитор'}));
    expect(mounts).toBe(1);
  });
});
