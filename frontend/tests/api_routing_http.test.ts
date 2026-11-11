import {afterAll, beforeAll, describe, expect, it} from 'bun:test';
import {createServer, loadConfigFromFile} from 'vite';

import type {ProxyOptions, ViteDevServer} from 'vite';

let vite: ViteDevServer;
let backend: ReturnType<typeof Bun.serve>;
let url: string;
beforeAll(async () => {
  backend = Bun.serve({hostname: '127.0.0.1', port: 0, fetch: async (request) => {
    const url = new URL(request.url);
    url.pathname = url.pathname.replace(/^\/api/, "");
    if (url.pathname === '/echo') {
      return Response.json({path: url.pathname, query: url.search, method: request.method,
        body: await request.text(), cookie: request.headers.get('cookie')});
    }
    if (url.pathname === '/posts/test/image') {
      const headers = {'content-type': 'image/png', etag: '"cover"'};
      return request.headers.has('if-none-match') ? new Response(null, {status: 304, headers})
        : new Response(new Uint8Array([0, 255, 128, 10]), {headers});
    }
    return new Response('backend missing', {status: 404});
  }});
  // The proxy rules come from vite.config.ts; only their localhost:8080 target is
  // redirected to this test backend.
  const configFile = `${import.meta.dir}/../../vite.config.ts`;
  const loaded = await loadConfigFromFile({command: 'serve', mode: 'development'}, configFile);
  const rules = loaded!.config.server!.proxy as Record<string, ProxyOptions>;
  expect(Object.keys(rules).sort()).toEqual(['^/api(?:/|$)', '^/ws(?:/|$)']);
  const target = `http://127.0.0.1:${backend.port}`;
  const proxy = Object.fromEntries(Object.entries(rules).map(([path, rule]) => {
    expect(rule.target).toBe('http://localhost:8080');
    return [path, {...rule, target}];
  }));
  vite = await createServer({configFile: false, ...loaded!.config,
    root: `${import.meta.dir}/../..`, optimizeDeps: {noDiscovery: true, include: []}, server: {host: '127.0.0.1', port: 0, proxy}});
  await vite.listen();
  const address = vite.httpServer!.address();
  if (!address || typeof address === 'string') {
    throw new Error('Missing Vite address');
  }
  url = `http://127.0.0.1:${address.port}`;
}, 30000);
afterAll(async () => {
  await vite?.close(); backend?.stop(true);
});

describe('Vite routing over HTTP', () => {
  it('forwards API paths, query, body and cookies', async () => {
    const response = await fetch(`${url}/api/echo?tag=a%2Fb&tag=c`, {
      method: 'POST', body: 'test-body', headers: {cookie: 'session_id=test'},
    });
    expect(await response.json()).toEqual({path: '/echo', query: '?tag=a%2Fb&tag=c',
      method: 'POST', body: 'test-body', cookie: 'session_id=test'});
  });
  it('preserves binary responses, conditional headers and API errors', async () => {
    const image = await fetch(`${url}/api/posts/test/image`);
    expect(image.headers.get('content-type')).toBe('image/png');
    expect(new Uint8Array(await image.arrayBuffer())).toEqual(new Uint8Array([0, 255, 128, 10]));
    const conditional = await fetch(`${url}/api/posts/test/image`, {headers: {'if-none-match': '"cover"'}});
    expect(conditional.status).toBe(304);
    expect(conditional.headers.get('etag')).toBe('"cover"');
    const missing = await fetch(`${url}/api/missing`);
    expect(missing.status).toBe(404);
    expect(await missing.text()).toBe('backend missing');
  });
  it('serves the SPA entry for profiles and nested task routes', async () => {
    for (const path of ['/@alice', '/school/contest/A', '/submissions/id', '/admin/users', '/apiary']) {
      const response = await fetch(url + path);
      expect(response.status).toBe(200);
      expect(await response.text()).toContain('/frontend/main.tsx');
    }
  });
});
