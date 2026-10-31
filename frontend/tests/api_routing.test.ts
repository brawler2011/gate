import {afterEach, describe, expect, it, spyOn} from 'bun:test';

import {getPostById} from "@/contracts/core/v1/api";
import {api} from '@/lib/api';

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe('SPA API transport', () => {
  it('uses /api and session cookies while retaining request options', async () => {
    const fetch = spyOn(globalThis, 'fetch').mockResolvedValue(Response.json({ok: true}));
    const controller = new AbortController();
    expect(await getPostById('post', {signal: controller.signal})).toEqual(expect.objectContaining({ok: true}));
    expect(fetch).toHaveBeenCalledWith('/api/posts/post', {credentials: 'include', signal: controller.signal, method: 'GET'});
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
  it('decodes the test validation report as JSON', async () => {
    const report = {total_tests: 2, valid_tests: 2, invalid_tests: 0, results: []};
    globalThis.fetch = (async () => Response.json(report)) as unknown as typeof fetch;
    const [error, data] = await api.validateAllTests('problem');
    expect(error).toBeNull();
    expect(data).toEqual(report);
  });
  it.each([
    ['generation', () => api.generateTests('problem', {generator_name: 'gen', test_numbers: [1]})],
    ['configuration', () => api.updateProblemTestsConfig('problem', {})],
    ['file update', () => api.updateProblemTestFile('problem', 'nested/edge.in', new Blob(['input']))],
  ] as const)('decodes the test %s response as JSON', async (_name, request) => {
    const response = {message: 'Updated'};
    globalThis.fetch = (async () => Response.json(response)) as unknown as typeof fetch;
    const [error, data] = await request();
    expect(error).toBeNull();
    expect(data).toEqual(response);
  });
  it.each(['text/plain', 'application/json'])('preserves test-file bytes for %s responses', async (contentType) => {
    const bytes = new Uint8Array([0, 255, 128, 10]);
    globalThis.fetch = (async () => new Response(bytes, {
      headers: {'Content-Type': contentType},
    })) as unknown as typeof fetch;
    const [error, data] = await api.getProblemTestFile('problem', 'nested/space #?.in');
    expect(error).toBeNull();
    expect(data).toBeInstanceOf(Blob);
    expect(new Uint8Array(await data!.arrayBuffer())).toEqual(bytes);
  });
  it('preserves status and request id on HTTP errors', async () => {
    globalThis.fetch = (async () => Response.json({message: 'Denied', request_id: 'req-1'}, {status: 403})) as unknown as typeof fetch;
    try {
      await getPostById('private');
      throw new Error('Expected HTTP failure');
    } catch (error) {
      expect(error).toBeInstanceOf(Error);
      expect((error as Error & {status: number}).status).toBe(403);
      expect((error as Error & {info: {request_id: string}}).info.request_id).toBe('req-1');
    }
  });
});
