import './setup-dom';

import {afterEach, expect, it, spyOn} from 'bun:test';

import MySubmissionsPage, {loader as myLoader} from '@/pages/contests/mysubmissions/MysubmissionsPage';
import SubmissionsPage, {loader} from '@/pages/contests/submissions/SubmissionsPage';

import {act, cleanup, renderWithProviders, waitFor} from './test-utils';

afterEach(cleanup);

it.each(['all', 'mine'])('keeps actual %s page selection in sync with HTTP and pagination', async mode => {
  const requests: URL[] = [];
  const fetch = spyOn(globalThis, 'fetch').mockImplementation(Object.assign(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost');
    if (url.pathname.endsWith('/users/me')) {
      return Response.json({user: {id: 'user', username: 'alice', role: 'admin'}});
    }
    if (url.pathname.endsWith('/my-role')) {
      return Response.json({role: 'owner'});
    }
    if (url.pathname.endsWith('/submissions')) {
      requests.push(url);
      const page = Number(url.searchParams.get('page'));
      return Response.json({submissions: [{id: `page-${page}`, username: `Page ${page}`, state: 200,
        contest_id: 'contest', contest_login: 'round', organization_login: 'school',
        problem_id: 'problem', problem_title: 'A+B', position: 1, language: 42,
        created_at: '2026-01-01T00:00:00Z'}], pagination: {page, total: 100}});
    }
    return Response.json({contest: {id: 'contest', login: 'round', organization_login: 'school', visibility: 'public'}, problems: []});
  }, {preconnect: () => {}}));
  const segment = mode === 'mine' ? 'mysubmissions' : 'submissions';
  const element = mode === 'mine' ? <MySubmissionsPage /> : <SubmissionsPage />;
  const suffix = '?state=0&language=42&order=asc&userId=other';
  try {
    const view = renderWithProviders(element, {
      route: `/school/round/${segment}${suffix}`,
      routes: [{path: `/:slug/:contestLogin/${segment}`, element, loader: mode === 'mine' ? myLoader : loader}],
    });
    await waitFor(() => expect(view.getByText('Page 1')).toBeTruthy());
    expect(requests).toHaveLength(1);
    expect(requests[0].searchParams.get('state')).toBe('0');
    expect(requests[0].searchParams.get('language')).toBe('42');
    expect(requests[0].searchParams.get('sortOrder')).toBe('asc');
    expect(requests[0].searchParams.get('userId')).toBe(mode === 'mine' ? 'user' : 'other');
    await act(async () => {
      await view.router.navigate(`/school/round/${segment}${suffix}&page=2`);
    });
    await waitFor(() => expect(view.getByText('Page 2')).toBeTruthy());
    expect(view.queryByText('Page 1')).toBeNull();
    expect(requests).toHaveLength(2);
    expect(requests[1].searchParams.get('page')).toBe('2');
    const page3 = view.getByRole('link', {name: '3'});
    expect(page3.getAttribute('href')).toContain('state=0');
    expect(page3.getAttribute('href')).toContain('language=42');
  } finally {
    fetch.mockRestore();
  }
});
