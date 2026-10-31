import {afterEach, describe, expect, it, spyOn} from 'bun:test';

import {api} from '@/lib/api';

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe('SPA API transport', () => {
  it('uses /api and session cookies while retaining request options', async () => {
    const fetch = spyOn(globalThis, 'fetch').mockResolvedValue(Response.json({ok: true}));
    const controller = new AbortController();
    const {data} = await api.GET('/posts/{id}', {params: {path: {id: 'post'}}, signal: controller.signal});
    expect(data).toEqual(expect.objectContaining({ok: true}));
    expect(fetch).toHaveBeenCalledWith('/api/posts/post', expect.objectContaining({credentials: 'include', signal: controller.signal, method: 'GET'}));
    fetch.mockRestore();
  });
  it('encodes path parameters and uses contract query names', async () => {
    const urls: string[] = [];
    globalThis.fetch = (async (url: string | URL | Request) => {
      urls.push(String(url));
      return Response.json({});
    }) as unknown as typeof fetch;
    await api.GET('/problems/{problemId}/tests/{name}', {params: {path: {problemId: 'problem', name: 'nested/space #?.in'}}});
    await api.GET('/problems', {
      params: {
        query: {
          pageSize: 10,
          organization_id: 'org',
        },
      },
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
      const {data, error} = await api.PATCH('/problems/{problemId}/statement', {
        params: {path: {problemId: 'problem/id'}, query: {lang: 'ru'}},
        body,
        signal: controller.signal,
      });
      expect(error).toBeUndefined();
      expect(data).toEqual(body);
      expect(fetch).toHaveBeenCalledWith('/api/problems/problem%2Fid/statement?lang=ru', expect.objectContaining({
        credentials: 'include',
        signal: controller.signal,
        method: 'PATCH',
        headers: expect.anything(),
        body: JSON.stringify(body),
      }));
    } finally {
      fetch.mockRestore();
    }
  });
  it('decodes the test validation report as JSON', async () => {
    const report = {total_tests: 2, valid_tests: 2, invalid_tests: 0, results: []};
    globalThis.fetch = (async () => Response.json(report)) as unknown as typeof fetch;
    const {data, error} = await api.POST('/problems/{problemId}/workshop/tests/validate', {params: {path: {problemId: 'problem'}}});
    expect(error).toBeUndefined();
    expect(data).toEqual(report);
  });
  it.each([
    ['generation', () => api.POST('/problems/{problemId}/workshop/tests/generate', {params: {path: {problemId: 'problem'}}, body: {generator_name: 'gen', test_numbers: [1]}})],
    ['configuration', () => api.PATCH('/problems/{problemId}/tests/config', {params: {path: {problemId: 'problem'}}, body: {}})],
    ['file update', () => api.PUT('/problems/{problemId}/tests/{name}', {params: {path: {problemId: 'problem', name: 'nested/edge.in'}}, body: new Blob(['input']) as unknown as string, headers: {'Content-Type': 'application/octet-stream'}})],
  ] as const)('decodes the test %s response as JSON', async (_name, request) => {
    const response = {message: 'Updated'};
    globalThis.fetch = (async () => Response.json(response)) as unknown as typeof fetch;
    const {data, error} = await request();
    expect(error).toBeUndefined();
    expect(data).toEqual(response);
  });
  it.each(['text/plain', 'application/json'])('preserves test-file bytes for %s responses', async (contentType) => {
    const bytes = new Uint8Array([0, 255, 128, 10]);
    globalThis.fetch = (async () => new Response(bytes, {
      headers: {'Content-Type': contentType},
    })) as unknown as typeof fetch;
    const {data, error} = await api.GET('/problems/{problemId}/tests/{name}', {params: {path: {problemId: 'problem', name: 'nested/space #?.in'}}, parseAs: 'blob'});
    expect(error).toBeUndefined();
    expect(data).toBeInstanceOf(Blob);
    expect(new Uint8Array(await data!.arrayBuffer())).toEqual(bytes);
  });
  it('preserves status and request id on HTTP errors', async () => {
    globalThis.fetch = (async () => Response.json({message: 'Denied', request_id: 'req-1'}, {status: 403})) as unknown as typeof fetch;
    const {data: _data, error, response} = await api.GET('/posts/{id}', {params: {path: {id: 'private'}}});
    expect(response.status).toBe(403);
    expect((error as {request_id: string}).request_id).toBe('req-1');
  });
});
