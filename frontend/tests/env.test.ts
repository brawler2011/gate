import {describe, expect, it} from 'bun:test';

import {env} from '../lib/env';

describe('Same-origin SPA URLs', () => {
  it('uses /api independently of server environment variables', () => {
    expect(env.getBackendApiUrl()).toBe('/api');
  });
  it('derives public and WebSocket URLs from the browser location', () => {
    const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
    try {
      for (const [origin, ws] of [['https://gate.test', 'wss://gate.test/ws'], ['http://localhost:5173', 'ws://localhost:5173/ws']]) {
        Object.defineProperty(globalThis, 'window', {value: {location: {origin}}, configurable: true});
        expect(env.getAppUrl()).toBe(origin!);
        expect(env.getWebSocketUrl()).toBe(ws!);
      }
    } finally {
      if (previous) {
        Object.defineProperty(globalThis, 'window', previous);
      } else {
        Reflect.deleteProperty(globalThis, 'window');
      }
    }
  });
});
