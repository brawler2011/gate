import {afterEach, describe, expect, it} from "bun:test";

import {env} from "../lib/env";

const originalEnv = {...process.env};
afterEach(() => {
  process.env = {...originalEnv};
});

describe("Required application environment", () => {
  for (const [name, getter] of [
    ["BACKEND_API_URL", env.getBackendApiUrl],
    ["NEXT_PUBLIC_WEBSOCKET_URL", env.getWebSocketUrl],
    ["NEXT_PUBLIC_APP_URL", env.getAppUrl],
  ] as const) {
    it(`${name} rejects missing and blank values`, () => {
      for (const value of [undefined, "", " \t\n"]) {
        if (value === undefined) {
          delete process.env[name];
        } else {
          process.env[name] = value;
        }
        expect(getter).toThrow(name);
      }
      process.env[name] = "https://gate.test";
      expect(getter()).toBe("https://gate.test");
    });
  }
});
