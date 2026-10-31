import {afterEach, describe, expect, it, spyOn} from 'bun:test';

import {api} from '@/lib/api';
import {apiFetch, HttpError} from '@/lib/fetch';

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe('SPA API transport', () => {
  it('uses /api and session cookies while retaining request options', async () => {
    const fetch = spyOn(globalThis, 'fetch').mockResolvedValue(Response.json({ok: true}));
    const controller = new AbortController();
    expect(await apiFetch('/posts?page=2', {signal: controller.signal})).toEqual({ok: true});
    expect(fetch).toHaveBeenCalledWith('/api/posts?page=2', {credentials: 'include', signal: controller.signal});
    fetch.mockRestore();
  });
  it('encodes path parameters and uses contract query names', async () => {
    const urls: string[] = [];
    globalThis.fetch = (async (url: string | URL | Request) => {
      urls.push(String(url));
      return Response.json({});
    }) as unknown as typeof fetch;
    await api.getProblemTestFile('problem', 'nested/space #?.in');
    await api.listProblems({
      pageSize: 10,
      organization_id: 'org'
    });
    expect(urls[0]).toBe('/api/problems/problem/tests/nested%2Fspace%20%23%3F.in');
    expect(urls[1]).toContain('organization_id=org');
    expect(urls[1]).toContain('pageSize=10');
  });
  it('passes path, body, query and request options through the facade', async () => {
    const body = {title: 'Updated statement', legend: 'Statement text', input_format: '', output_format: ''};
    const fetch = spyOn(globalThis, 'fetch').mockResolvedValue(Response.json(body));
    try {
      const controller = new AbortController();
      const [error, data] = await api.updateProblemStatement(
        'problem/id', body, {lang: 'ru'}, {signal: controller.signal}
      );
      expect(error).toBeNull();
      expect(data).toEqual(body);
      expect(fetch).toHaveBeenCalledWith('/api/problems/problem%2Fid/statement?lang=ru', {
        credentials: 'include',
        signal: controller.signal,
        method: 'PATCH',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(body),
      });
    } finally {
      fetch.mockRestore();
    }
  });
  it('preserves status and request id on HTTP errors', async () => {
    globalThis.fetch = (async () => Response.json({message: 'Denied', request_id: 'req-1'}, {status: 403})) as unknown as typeof fetch;
    try {
      await apiFetch('/private');
      throw new Error('Expected HTTP failure');
    } catch (error) {
      expect(error).toBeInstanceOf(HttpError);
      expect((error as HttpError).status).toBe(403);
      expect((error as HttpError).body.request_id).toBe('req-1');
    }
  });
});
