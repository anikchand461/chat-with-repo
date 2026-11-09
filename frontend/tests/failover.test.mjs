// Tests for the AWS -> Render failover in js/app.js (apiFetch).
// Run with: node --test frontend/tests/
//
// app.js is a browser script, so the API config + apiFetch block is pulled
// out of it and run in a VM against a fake fetch. The 10s timeout is shrunk
// to keep the suite fast.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";

const SOURCE = readFileSync(new URL("../js/app.js", import.meta.url), "utf8");
const between = (from, to) => SOURCE.slice(SOURCE.indexOf(from), SOURCE.indexOf(to));
const BLOCK =
  between("// AWS is the primary backend", "// Change this to your repository") +
  between("// POSTs that are safe to send twice", "async function request(");

const AWS = "https://qettzfl73r7xhbres3oqvx3qt40mmqrp.lambda-url.ap-south-1.on.aws";
const RENDER = "https://chat-with-repo-4vwy.onrender.com";
const TIMEOUT = 150;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// routes: { "<backend>[ <path>]": behaviour } where behaviour is a status
// number, "hang", "neterr", or { status, delay }. Pass the same `store` to
// a second setup() to simulate the next page load in the same browser.
function setup(routes, store = new Map()) {
  const calls = [];
  let aborted = 0;

  const fakeFetch = (url, init = {}) => {
    const base = url.startsWith(AWS) ? "AWS" : "RENDER";
    const path = url.slice(base === "AWS" ? AWS.length : RENDER.length);
    calls.push({ base, path, method: init.method || "GET", headers: init.headers, body: init.body });

    let b = routes[`${base} ${path}`] ?? routes[base] ?? 200;
    if (typeof b === "number") b = { status: b, delay: 0 };

    return new Promise((resolve, reject) => {
      init.signal?.addEventListener("abort", () => {
        aborted++;
        reject(new DOMException("aborted", "AbortError"));
      });
      if (b === "hang") return;
      if (b === "neterr") return reject(new TypeError("Failed to fetch"));
      setTimeout(
        () => resolve({ status: b.status, ok: b.status < 400, body: { cancel: async () => {} } }),
        b.delay
      );
    });
  };

  const localStorage = {
    getItem: (k) => (store.has(k) ? store.get(k) : null),
    setItem: (k, v) => store.set(k, String(v)),
    removeItem: (k) => store.delete(k),
  };
  const listeners = {};
  const addEventListener = (name, fn) => (listeners[name] ??= []).push(fn);
  const ctx = vm.createContext({ fetch: fakeFetch, setTimeout, clearTimeout, AbortController, DOMException, Set, Promise, Date, Number, String, localStorage, addEventListener });
  const code = BLOCK.replace(/PRIMARY_TIMEOUT_MS = \d+/, `PRIMARY_TIMEOUT_MS = ${TIMEOUT}`);
  vm.runInContext(code + "\nthis.apiFetch = apiFetch;", ctx);

  const real = (c) => c.path !== "/"; // ignore health probes
  return {
    apiFetch: ctx.apiFetch,
    calls: () => calls.filter(real).map((c) => `${c.method} ${c.base}${c.path}`),
    raw: () => calls,
    aborted: () => aborted,
    leavePage: () => listeners.pagehide.forEach((fn) => fn()),
    store,
  };
}

const POST = (body) => ({
  method: "POST",
  headers: { "Content-Type": "application/json", Authorization: "Bearer t0k3n" },
  body: JSON.stringify(body),
});

test("timeout is 10 seconds and AWS is primary", () => {
  assert.match(SOURCE, /const PRIMARY_TIMEOUT_MS = 10000;/);
  assert.match(SOURCE, new RegExp(`const API_PRIMARY = "${AWS}";`));
  assert.match(SOURCE, /const USE_LOCAL_API = false;/);
  assert.match(BLOCK, /\[API_PRIMARY, API_FALLBACK\]/);
});

test("healthy AWS: one request, no added delay", async () => {
  const s = setup({ AWS: 200 });
  const t0 = Date.now();
  const r = await s.apiFetch("/chat/list", {});
  assert.equal(r.status, 200);
  assert.ok(Date.now() - t0 < 50);
  assert.deepEqual(s.calls(), ["GET AWS/chat/list"]);
});

test("AWS hangs: GET aborted after timeout, Render gets it", async () => {
  const s = setup({ AWS: "hang", RENDER: 200 });
  const t0 = Date.now();
  const r = await s.apiFetch("/chat/list", {});
  const took = Date.now() - t0;
  assert.equal(r.status, 200);
  assert.ok(took >= TIMEOUT && took < TIMEOUT + 80, `took ${took}ms`);
  assert.deepEqual(s.calls(), ["GET AWS/chat/list", "GET RENDER/chat/list"]);
  assert.equal(s.aborted(), 1);
});

test("AWS slow but answers within timeout: no fallback", async () => {
  const s = setup({ AWS: { status: 200, delay: TIMEOUT / 2 } });
  const r = await s.apiFetch("/auth/me", {});
  assert.equal(r.status, 200);
  assert.deepEqual(s.calls(), ["GET AWS/auth/me"]);
});

test("network error falls back immediately", async () => {
  const s = setup({ AWS: "neterr", RENDER: 200 });
  const t0 = Date.now();
  await s.apiFetch("/chat/list", {});
  assert.ok(Date.now() - t0 < 50);
  assert.deepEqual(s.calls(), ["GET AWS/chat/list", "GET RENDER/chat/list"]);
});

test("requests cancelled by leaving the page don't fail over or mark AWS down", async () => {
  const s = setup({ AWS: "neterr", RENDER: 200 });
  s.leavePage();
  await assert.rejects(s.apiFetch("/auth/me", {}), { name: "AbortError" });
  await assert.rejects(s.apiFetch("/chat/create", POST({ owner: "o", repo: "r" })), { name: "AbortError" });
  assert.deepEqual(s.calls(), ["GET AWS/auth/me", "POST AWS/chat/create"]);
  assert.equal(s.store.size, 0, "AWS must not be marked down");
});

for (const status of [400, 401, 403, 404, 409, 422, 500]) {
  test(`AWS ${status}: returned as-is, no fallback`, async () => {
    const s = setup({ AWS: status });
    const r = await s.apiFetch("/auth/login", POST({}));
    assert.equal(r.status, status);
    assert.deepEqual(s.calls(), ["POST AWS/auth/login"]);
  });
}

for (const status of [502, 503, 504]) {
  test(`AWS ${status} on GET: falls back`, async () => {
    const s = setup({ AWS: status, RENDER: 200 });
    assert.equal((await s.apiFetch("/chat/list", {})).status, 200);
    assert.deepEqual(s.calls(), ["GET AWS/chat/list", "GET RENDER/chat/list"]);
  });
}

test("AWS throttled (Lambda paused, 429 everywhere): reads and creates go to Render", async () => {
  const s = setup({ AWS: 429, RENDER: 200 });
  assert.equal((await s.apiFetch("/chat/list", {})).status, 200);
  assert.equal(s.store.size, 1, "AWS should be marked down");

  const t = setup({ AWS: 429, RENDER: 201 });
  assert.equal((await t.apiFetch("/chat/create", POST({ owner: "o", repo: "r" }))).status, 201);
  assert.deepEqual(t.calls(), ["POST AWS/chat/create", "POST RENDER/chat/create"]);
});

test("504 on non-idempotent POST: no fallback (AWS may still be working)", async () => {
  const s = setup({ "AWS /chat/create": 504 });
  assert.equal((await s.apiFetch("/chat/create", POST({}))).status, 504);
  assert.deepEqual(s.calls(), ["POST AWS/chat/create"]);
});

test("login falls back with identical method, headers and body", async () => {
  const s = setup({ AWS: "hang", RENDER: 200 });
  const init = POST({ email: "a@b.c", password: "x" });
  await s.apiFetch("/auth/login", init);
  const [aws, render] = s.raw().filter((c) => c.path !== "/");
  assert.equal(render.base, "RENDER");
  assert.equal(render.method, "POST");
  assert.equal(render.body, init.body);
  assert.deepEqual(render.headers, init.headers);
  assert.deepEqual(render.headers, aws.headers);
  assert.equal(Object.keys(render.headers).filter((k) => /^authorization$/i.test(k)).length, 1);
});

test("Google sign-in is retry-safe: AWS 504 or hang -> same credential to Render", async () => {
  for (const AWS_BEHAVIOUR of [504, "hang"]) {
    const s = setup({ "AWS /auth/google": AWS_BEHAVIOUR, RENDER: 200 });
    const init = POST({ credential: "google-id-token" });
    assert.equal((await s.apiFetch("/auth/google", init)).status, 200);
    assert.deepEqual(s.calls(), ["POST AWS/auth/google", "POST RENDER/auth/google"]);
    assert.equal(s.raw().at(-1).body, init.body);
  }
});

test("stream: AWS down (hang + probe fails) -> Render within timeout", async () => {
  const s = setup({ AWS: "hang", RENDER: 200 });
  const t0 = Date.now();
  const r = await s.apiFetch("/chat/1/ask/stream", POST({ question: "q" }));
  assert.equal(r.status, 200);
  assert.ok(Date.now() - t0 < TIMEOUT + 80);
  assert.deepEqual(s.calls(), ["POST AWS/chat/1/ask/stream", "POST RENDER/chat/1/ask/stream"]);
});

test("stream: AWS alive but slow to start -> keep waiting, never duplicated", async () => {
  const s = setup({ "AWS /": 200, "AWS /chat/1/ask/stream": { status: 200, delay: TIMEOUT * 3 } });
  const r = await s.apiFetch("/chat/1/ask/stream", POST({ question: "q" }));
  assert.equal(r.status, 200);
  assert.deepEqual(s.calls(), ["POST AWS/chat/1/ask/stream"]);
  assert.equal(s.aborted(), 0);
});

test("stream: no abort after headers arrive (long stream is safe)", async () => {
  const s = setup({ AWS: 200 });
  await s.apiFetch("/chat/1/ask/stream", POST({ question: "q" }));
  await sleep(TIMEOUT * 2);
  assert.equal(s.aborted(), 0);
});

test("chat create: network error while AWS alive -> throws, no duplicate", async () => {
  const s = setup({ "AWS /": 200, "AWS /chat/create": "neterr" });
  await assert.rejects(s.apiFetch("/chat/create", POST({})), /Failed to fetch/);
  assert.deepEqual(s.calls(), ["POST AWS/chat/create"]);
});

test("chat create: AWS app down behind proxy (probe 502) -> Render", async () => {
  const s = setup({ "AWS /": 502, "AWS /chat/create": "neterr", RENDER: 201 });
  assert.equal((await s.apiFetch("/chat/create", POST({}))).status, 201);
  assert.deepEqual(s.calls(), ["POST AWS/chat/create", "POST RENDER/chat/create"]);
});

const DOWN_KEY = "devlens_api_primary_down_until";

test("after a failover, later requests skip AWS with no wait", async () => {
  const s = setup({ AWS: "hang", RENDER: 200 });
  await s.apiFetch("/chat/list", {});
  assert.ok(Number(s.store.get(DOWN_KEY)) > Date.now());

  const t0 = Date.now();
  await s.apiFetch("/chat/1/index-status", {});
  await s.apiFetch("/chat/1/messages", {});
  assert.ok(Date.now() - t0 < 50);
  assert.deepEqual(s.calls(), [
    "GET AWS/chat/list",
    "GET RENDER/chat/list",
    "GET RENDER/chat/1/index-status",
    "GET RENDER/chat/1/messages",
  ]);
});

test("skip survives a page reload (shared localStorage)", async () => {
  const first = setup({ AWS: "hang", RENDER: 200 });
  await first.apiFetch("/chat/list", {});

  const next = setup({ AWS: "hang", RENDER: 200 }, first.store);
  const t0 = Date.now();
  await next.apiFetch("/chat/1/ask/stream", POST({ question: "q" }));
  assert.ok(Date.now() - t0 < 50);
  assert.deepEqual(next.calls(), ["POST RENDER/chat/1/ask/stream"]);
});

test("while skipped, one background check per page brings AWS back", async () => {
  const store = new Map([[DOWN_KEY, String(Date.now() + 60000)]]);
  const s = setup({ AWS: 200, RENDER: 200 }, store);
  await s.apiFetch("/chat/list", {}); // skipped; starts the background check
  await sleep(10);
  assert.equal(store.has(DOWN_KEY), false);

  await s.apiFetch("/auth/me", {});
  assert.deepEqual(s.calls(), ["GET RENDER/chat/list", "GET AWS/auth/me"]);
  assert.equal(s.raw().filter((c) => c.base === "AWS" && c.path === "/").length, 1);
});

test("skip expires and AWS is tried first again", async () => {
  const store = new Map([[DOWN_KEY, String(Date.now() - 1)]]);
  const s = setup({ AWS: 200 }, store);
  await s.apiFetch("/chat/list", {});
  assert.deepEqual(s.calls(), ["GET AWS/chat/list"]);
});

test("4xx and unsafe-POST-while-alive do not mark AWS down", async () => {
  const a = setup({ AWS: 401 });
  await a.apiFetch("/auth/me", {});
  assert.equal(a.store.has(DOWN_KEY), false);

  const b = setup({ "AWS /": 200, "AWS /chat/create": "neterr" });
  await assert.rejects(b.apiFetch("/chat/create", POST({})));
  assert.equal(b.store.has(DOWN_KEY), false);
});

test("502 marks AWS down", async () => {
  const s = setup({ AWS: 502, RENDER: 200 });
  await s.apiFetch("/chat/list", {});
  assert.ok(s.store.has(DOWN_KEY));
});
