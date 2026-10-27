import {mkdtemp, mkdir, readFile, rm, writeFile} from "node:fs/promises";
import {join} from "node:path";

import {afterAll, beforeAll, describe, expect, it} from "bun:test";
import sharp from "sharp";

const frontendRoot = join(import.meta.dir, "..");
let fixtureRoot: string;
let frontend: ReturnType<typeof Bun.spawn>;
let backend: ReturnType<typeof Bun.serve>;
let frontendUrl: string;
let serverOutput = "";

beforeAll(async () => {
  const png = new Uint8Array(await sharp({
    create: {width: 96, height: 80, channels: 3, background: "#ab3322"},
  }).png().toBuffer());
  backend = Bun.serve({
    hostname: "127.0.0.1",
    port: 0,
    fetch: async (request) => {
      const url = new URL(request.url);
      if (url.pathname === "/posts/test/image") {
        const headers = {"content-type": "image/png", etag: '"cover"'};
        return request.headers.get("if-none-match") === '"cover"'
          ? new Response(null, {status: 304, headers})
          : new Response(png, {headers});
      }
      if (url.pathname === "/echo") {
        return Response.json({
          path: url.pathname, query: url.search, method: request.method,
          body: await request.text(), cookie: request.headers.get("cookie"),
        });
      }
      return new Response("backend missing", {status: 404});
    },
  });

  // Keep the fixture under frontend so Next.js resolves the installed dependencies.
  fixtureRoot = await mkdtemp(join(frontendRoot, "api-routing-test-"));
  await mkdir(join(fixtureRoot, "lib"));
  await mkdir(join(fixtureRoot, "app/user/[username]"), {recursive: true});
  for (const file of ["proxy.ts", "lib/env.ts"]) {
    await writeFile(join(fixtureRoot, file), await readFile(join(frontendRoot, file)));
  }
  await writeFile(join(fixtureRoot, "package.json"), JSON.stringify({name: "api-routing-test", private: true}));
  await writeFile(join(fixtureRoot, "tsconfig.json"), JSON.stringify({
    compilerOptions: {paths: {"@/*": ["./*"]}},
  }));
  await writeFile(join(fixtureRoot, "app/layout.tsx"),
    'export default function Layout({children}) { return <html><body>{children}</body></html>; }');
  await writeFile(join(fixtureRoot, "app/page.tsx"), 'export default function Page() { return <p>home</p>; }');
  await writeFile(join(fixtureRoot, "app/user/[username]/page.tsx"),
    'export default async function Page({params}) { return <p>profile:{(await params).username}</p>; }');

  const reservation = Bun.serve({hostname: "127.0.0.1", port: 0, fetch: () => new Response()});
  const port = reservation.port;
  reservation.stop(true);
  frontendUrl = `http://localhost:${port}`;
  frontend = Bun.spawn([
    "bun", join(frontendRoot, "node_modules/next/dist/bin/next"),
    "dev", fixtureRoot, "--hostname", "localhost", "--port", String(port),
  ], {
    cwd: frontendRoot,
    env: {...process.env, BACKEND_API_URL: `http://127.0.0.1:${backend.port}`, NEXT_TELEMETRY_DISABLED: "1"},
    stdout: "pipe", stderr: "pipe",
  });
  const collectOutput = async (stream: ReadableStream<Uint8Array>) => {
    const reader = stream.getReader();
    while (true) {
      const {done, value} = await reader.read();
      if (done) {
        break;
      }
      serverOutput += new TextDecoder().decode(value);
    }
  };
  void collectOutput(frontend.stdout as ReadableStream<Uint8Array>);
  void collectOutput(frontend.stderr as ReadableStream<Uint8Array>);
  for (let attempt = 0; attempt < 120; attempt++) {
    if (frontend.exitCode !== null) {
      throw new Error(serverOutput);
    }
    try {
      const response = await fetch(`${frontendUrl}/api/echo`, {signal: AbortSignal.timeout(1000)});
      if (response.ok) {
        return;
      }
    } catch {
      // The server may still be starting or compiling its proxy.
    }
    await Bun.sleep(250);
  }
  throw new Error(`Next.js did not become ready: ${serverOutput}`);
}, 60000);

afterAll(async () => {
  if (frontend) {
    frontend.kill();
    await frontend.exited;
  }
  backend?.stop(true);
  if (fixtureRoot) {
    await rm(fixtureRoot, {recursive: true, force: true});
  }
});

describe("API routing over HTTP", () => {
  it("optimizes a backend image through the internal /api route", async () => {
    const response = await fetch(`${frontendUrl}/_next/image?url=%2Fapi%2Fposts%2Ftest%2Fimage&w=640&q=75`, {
      headers: {accept: "image/webp"},
    });
    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("image/webp");
    const image = await sharp(Buffer.from(await response.arrayBuffer())).metadata();
    expect([image.format, image.width, image.height]).toEqual(["webp", 96, 80]);
  }, 30000);

  it("forwards the path, query, method, body, and cookie", async () => {
    const response = await fetch(`${frontendUrl}/api/echo?tag=a%2Fb&tag=c`, {
      method: "POST", body: "test-body", headers: {cookie: "session_id=test"},
    });
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      path: "/echo", query: "?tag=a%2Fb&tag=c", method: "POST",
      body: "test-body", cookie: "session_id=test",
    });
  });

  it("preserves conditional headers and backend error responses", async () => {
    const conditional = await fetch(`${frontendUrl}/api/posts/test/image`, {headers: {"if-none-match": '"cover"'}});
    expect(conditional.status).toBe(304);
    expect(conditional.headers.get("etag")).toBe('"cover"');
    const missing = await fetch(`${frontendUrl}/api/missing`);
    expect(missing.status).toBe(404);
    expect(await missing.text()).toBe("backend missing");
  });

  it("preserves public profile rewrites and blocks direct internal profile access", async () => {
    const profile = await fetch(`${frontendUrl}/@alice`);
    if (profile.status !== 200) {
      throw new Error(`Profile ${profile.status}: ${serverOutput}`);
    }
    expect(await profile.text()).toContain("profile:<!-- -->alice");
    expect((await fetch(`${frontendUrl}/user/alice`)).status).toBe(404);
  }, 30000);
});
