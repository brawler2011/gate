import {mkdtemp, rm} from "node:fs/promises";
import {tmpdir} from "node:os";
import {join} from "node:path";

import {afterAll, beforeAll, expect, it} from "bun:test";

const frontendRoot = join(import.meta.dir, "..");
const backendRoot = join(frontendRoot, "../backend");
const problemId = "00000000-0000-0000-0000-000000000038";
let fixtureRoot: string;
let backend: ReturnType<typeof Bun.spawn>;
let backendUrl: string;

beforeAll(async () => {
  fixtureRoot = await mkdtemp(join(tmpdir(), "gate-workshop-http-"));
  const binary = join(fixtureRoot, "workshop-http");
  const build = Bun.spawn(["go", "build", "-o", binary, "./tests/fixtures/workshop-http"], {
    cwd: backendRoot, stdout: "ignore", stderr: "pipe",
  });
  const buildError = await new Response(build.stderr).text();
  expect(await build.exited, buildError).toBe(0);
  const reservation = Bun.serve({hostname: "127.0.0.1", port: 0, fetch: () => new Response()});
  const address = `127.0.0.1:${reservation.port}`;
  reservation.stop(true);
  backendUrl = `http://${address}`;
  backend = Bun.spawn([binary, fixtureRoot, address, problemId], {stdout: "ignore", stderr: "pipe"});
  for (let attempt = 0; attempt < 100; attempt++) {
    if (backend.exitCode !== null) {
      throw new Error(await new Response(backend.stderr as ReadableStream<Uint8Array>).text());
    }
    try {
      if ((await fetch(`${backendUrl}/problems/${problemId}/tests`)).ok) {
        return;
      }
    } catch {
      // The fixture has not started listening yet.
    }
    await Bun.sleep(25);
  }
  throw new Error("Workshop HTTP fixture did not become ready");
}, 300000);

afterAll(async () => {
  if (backend) {
    backend.kill(); await backend.exited;
  }
  if (fixtureRoot) {
    await rm(fixtureRoot, {recursive: true, force: true});
  }
});

it("serializes raw nested filenames through the production clients and real backend router without losing bytes", async () => {
  // A fresh process initializes the actual facade with this fixture's base URL;
  // no service method, request serializer, fetch call or router is mocked.
  const source = `
import assert from "node:assert/strict";

import {api} from "./lib/api";
const browserFetch = globalThis.fetch;
globalThis.fetch = ((url, options) => browserFetch(new URL(String(url).slice(4), ${JSON.stringify(backendUrl)}), options));

const problemId = ${JSON.stringify(problemId)};
for (const name of ["nested/edge.in", "nested/edge.ans", "nested/space #?.in"]) {
  let expected = new Uint8Array([0, 255, 128, 10]);
  for (const client of [api]) {
    const {data, error: readError} = await client.GET("/problems/{problemId}/tests/{name}", {
      params: {path: {problemId, name}},
      parseAs: "blob",
    });
    assert.equal(readError, undefined);
    assert.ok(data instanceof Blob);
    assert.deepEqual(new Uint8Array(await data.arrayBuffer()), expected);
    expected = new Uint8Array([0, 254, 129, 10, expected.length]);
    const {error: writeError} = await client.PUT("/problems/{problemId}/tests/{name}", {
      params: {path: {problemId, name}},
      body: new Blob([expected]) as unknown as string,
      bodySerializer: (b) => b as unknown as BodyInit,
    });
    assert.equal(writeError, undefined);
    const {data: saved, error: savedError} = await client.GET("/problems/{problemId}/tests/{name}", {
      params: {path: {problemId, name}},
      parseAs: "blob",
    });
    assert.equal(savedError, undefined);
    assert.ok(saved instanceof Blob);
    assert.deepEqual(new Uint8Array(await saved.arrayBuffer()), expected);
  }
}
const {response: unsafeRead} = await api.GET("/problems/{problemId}/tests/{name}", {
  params: {path: {problemId, name: "../problem.yaml"}},
  parseAs: "blob",
});
assert.equal(unsafeRead.status, 400);
const {response: unsafeWrite} = await api.PUT("/problems/{problemId}/tests/{name}", {
  params: {path: {problemId, name: "../outside.in"}},
  body: new Blob(["unsafe"]) as unknown as string,
  bodySerializer: (b) => b as unknown as BodyInit,
});
assert.equal(unsafeWrite.status, 400);
const {error: deleteError} = await api.DELETE("/problems/{problemId}/tests/{name}", {
  params: {path: {problemId, name: "nested/space #?.in"}},
});
assert.equal(deleteError, undefined);
const {response: missing} = await api.GET("/problems/{problemId}/tests/{name}", {
  params: {path: {problemId, name: "nested/space #?.in"}},
  parseAs: "blob",
});
assert.equal(missing.status, 404);
`;
  const client = Bun.spawn(["bun", "--eval", source], {
    cwd: frontendRoot, env: {...process.env, BACKEND_API_URL: backendUrl}, stdout: "pipe", stderr: "pipe",
  });
  const stderr = await new Response(client.stderr).text();
  expect(await client.exited, stderr).toBe(0);
});
