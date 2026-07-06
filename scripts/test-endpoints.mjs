#!/usr/bin/env node
// SLI Backend — endpoint smoke test.
//
// Deliberately NOT part of the Go module/build (lives in scripts/, plain
// Node.js). Requires Node 18+ (uses built-in fetch, no npm install).
//
// Run:
//   node scripts/test-endpoints.mjs
//
// Config (env vars, all optional):
//   BASE_URL       default http://localhost:8080
//   ACCESS_TOKEN   a real JWT access token, to also test authenticated GETs.
//                  Get one via GET /api/auth/google/login -> Google login ->
//                  callback sets an `access_token` cookie; copy its value
//                  from your browser's dev tools (Application > Cookies).
//   VERBOSE        "1" to print full response bodies on failure
//
// Why Authorization: Bearer instead of cookies: internal/http/middleware/csrf.go
// only enforces CSRF for cookie-based auth (see usesCookieAuth/hasBearerToken) -
// sending the JWT as `Authorization: Bearer <token>` instead of a cookie skips
// CSRF entirely, which is much simpler for a script than doing the double-submit
// cookie dance. This only works for GET/read endpoints tested here; the app
// itself must keep using cookies + CSRF for real browser traffic.
//
// Scope: this is a WIRING smoke test, not a full functional/workflow test.
// It verifies:
//   1. Public endpoints respond (health checks, OAuth login redirect).
//   2. Every authenticated route correctly rejects requests with no token
//      (401) - safe, zero side effects, always runs.
//   3. If ACCESS_TOKEN is set, the safe read-only authenticated endpoints
//      also get hit with the real token, so you can see 200 vs 403 (role
//      mismatch) vs 401 (something's actually broken).
// It does NOT create/mutate data (no enroll/assign/submit calls) - those
// need real IDs from your database and are commented at the bottom as a
// starting point if you want to extend this.

const BASE_URL = (process.env.BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const ACCESS_TOKEN = process.env.ACCESS_TOKEN || "";
const VERBOSE = process.env.VERBOSE === "1";

const RESET = "\x1b[0m", GREEN = "\x1b[32m", RED = "\x1b[31m", YELLOW = "\x1b[33m", DIM = "\x1b[2m", BOLD = "\x1b[1m";

const results = [];

async function timedFetch(method, path, { auth = false, redirect = "follow", body } = {}) {
  const headers = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth && ACCESS_TOKEN) headers["Authorization"] = `Bearer ${ACCESS_TOKEN}`;

  const start = Date.now();
  try {
    const res = await fetch(BASE_URL + path, {
      method,
      headers,
      redirect,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    const ms = Date.now() - start;
    let text = "";
    try { text = await res.text(); } catch { /* ignore */ }
    return { status: res.status, ms, text, ok: true };
  } catch (err) {
    const ms = Date.now() - start;
    return { status: 0, ms, text: String(err), ok: false };
  }
}

// judge(status) -> { pass, note }
function record(method, path, label, res, judge) {
  const { pass, note } = judge(res.status);
  results.push({ method, path, label, status: res.status, ms: res.ms, pass, note, body: res.text });
}

function printResults() {
  const col = (s, n) => String(s).padEnd(n);
  console.log(`\n${BOLD}${col("METHOD", 7)}${col("PATH", 55)}${col("STATUS", 8)}${col("TIME", 9)}RESULT${RESET}`);
  console.log(DIM + "-".repeat(100) + RESET);
  let pass = 0, fail = 0, skip = 0;
  for (const r of results) {
    let tag;
    if (r.note === "SKIP") { tag = `${YELLOW}SKIP${RESET}`; skip++; }
    else if (r.pass) { tag = `${GREEN}PASS${RESET}`; pass++; }
    else { tag = `${RED}FAIL${RESET}`; fail++; }
    console.log(
      `${col(r.method, 7)}${col(r.label, 55)}${col(r.status || "-", 8)}${col(r.ms ? r.ms.toFixed(0) + "ms" : "-", 9)}${tag}${r.note && r.note !== "SKIP" ? "  " + DIM + r.note + RESET : ""}`
    );
    if (!r.pass && r.note !== "SKIP" && VERBOSE) {
      console.log(DIM + "  body: " + r.body.slice(0, 300).replace(/\n/g, " ") + RESET);
    }
  }
  console.log(DIM + "-".repeat(100) + RESET);
  console.log(`${BOLD}${pass} passed, ${fail} failed, ${skip} skipped${RESET} against ${BASE_URL}\n`);
  return fail === 0;
}

// Every route under the authenticated group in internal/http/router.go.
// method, path, a placeholder body for POST/PUT where the handler decodes JSON.
const AUTH_ROUTES = [
  ["GET",  "/api/auth/csrf"],
  ["POST", "/api/auth/logout"],
  ["GET",  "/api/auth/me"],
  ["POST", "/api/coordinator/internships", { student_id: "00000000-0000-0000-0000-000000000000" }],
  ["POST", "/api/coordinator/internships/assign", { internship_id: "00000000-0000-0000-0000-000000000000", faculty_id: "00000000-0000-0000-0000-000000000000" }],
  ["GET",  "/api/coordinator/internships"],
  ["POST", "/api/student/reports/weekly/1", { content: "test" }],
  ["PUT",  "/api/student/reports/weekly/1", { content: "test" }],
  ["GET",  "/api/student/reports"],
  ["GET",  "/api/faculty/students"],
  ["POST", "/api/faculty/students/approve", { assignment_id: "00000000-0000-0000-0000-000000000000" }],
  ["POST", "/api/faculty/reports/00000000-0000-0000-0000-000000000000/feedback", { comments: "test" }],
  ["POST", "/api/faculty/evaluations/00000000-0000-0000-0000-000000000000/schedule", { venue: "test", in_semester_at: "2026-01-01", end_semester_at: "2026-01-02" }],
  ["POST", "/api/faculty/evaluations/00000000-0000-0000-0000-000000000000/submit", {}],
  ["GET",  "/api/faculty/evaluations/00000000-0000-0000-0000-000000000000/marksheet/download"],
  ["GET",  "/api/hod/statistics"],
  ["GET",  "/api/hod/overview"],
  ["GET",  "/api/admin/audit-logs"],
  ["GET",  "/api/admin/users"],
  ["POST", "/api/admin/evaluations/00000000-0000-0000-0000-000000000000/correct", { reason: "test" }],
];

// Safe (read-only, idempotent) subset worth re-hitting with a real token.
// Keyed on "METHOD path", not path alone - several paths (e.g.
// /api/coordinator/internships) are used by both a safe GET and a mutating
// POST, and a path-only filter would incorrectly match both.
// Deliberately excludes /api/auth/logout (would invalidate ACCESS_TOKEN for
// the rest of the run) and every mutating POST/PUT.
const SAFE_READ_ROUTES = new Set([
  "GET /api/auth/csrf",
  "GET /api/auth/me",
  "GET /api/coordinator/internships",
  "GET /api/student/reports",
  "GET /api/faculty/students",
  "GET /api/hod/statistics",
  "GET /api/hod/overview",
  "GET /api/admin/audit-logs",
  "GET /api/admin/users",
]);

async function main() {
  console.log(`${BOLD}Testing ${BASE_URL}${RESET}${ACCESS_TOKEN ? "" : DIM + "  (no ACCESS_TOKEN set - only public + no-auth-401 checks run)" + RESET}`);

  // 1. Public endpoints - always run, concurrently.
  await Promise.all([
    (async () => {
      const res = await timedFetch("GET", "/healthz");
      record("GET", "/healthz", "/healthz", res, (s) => ({ pass: s === 200, note: s === 200 ? "" : `expected 200` }));
    })(),
    (async () => {
      const res = await timedFetch("GET", "/readyz");
      record("GET", "/readyz", "/readyz", res, (s) => ({ pass: s === 200, note: s === 200 ? "" : `expected 200 (DB not reachable?)` }));
    })(),
    (async () => {
      const res = await timedFetch("GET", "/api/auth/google/login", { redirect: "manual" });
      record("GET", "/api/auth/google/login", "/api/auth/google/login", res, (s) => ({
        pass: s === 307 || s === 302 || (s >= 300 && s < 400),
        note: (s >= 300 && s < 400) ? "" : `expected a redirect to Google`,
      }));
    })(),
    (async () => {
      const res = await timedFetch("GET", "/api/auth/google/callback");
      record("GET", "/api/auth/google/callback (no code)", "/api/auth/google/callback", res, (s) => ({
        pass: s === 401 || s === 400,
        note: (s === 401 || s === 400) ? "" : `expected 400/401 without a real oauth code/state`,
      }));
    })(),
  ]);

  // 2. Every authenticated route, with NO token - expect 401. Always runs,
  // zero side effects (Auth middleware rejects before any handler runs).
  await Promise.all(
    AUTH_ROUTES.map(async ([method, path, body]) => {
      const res = await timedFetch(method, path, { body });
      record(method, path, `${path}  [no token]`, res, (s) => ({
        pass: s === 401,
        note: s === 401 ? "" : `expected 401 - auth middleware may not be wired for this route`,
      }));
    })
  );

  // 3. With a real ACCESS_TOKEN: re-hit the safe read-only endpoints.
  if (ACCESS_TOKEN) {
    await Promise.all(
      AUTH_ROUTES.filter(([method, path]) => SAFE_READ_ROUTES.has(`${method} ${path}`)).map(async ([method, path]) => {
        const res = await timedFetch(method, path, { auth: true });
        record(method, path, `${path}  [with token]`, res, (s) => {
          if (s === 200) return { pass: true, note: "" };
          if (s === 403) return { pass: true, note: "403 = wrong role for this token (expected unless token has this role)" };
          if (s === 401) return { pass: false, note: "401 with a token set - token invalid/expired, or auth is broken" };
          return { pass: false, note: `unexpected status` };
        });
      })
    );
  } else {
    for (const [method, path] of AUTH_ROUTES) {
      if (SAFE_READ_ROUTES.has(`${method} ${path}`)) {
        results.push({ method, path, label: `${path}  [with token]`, status: "-", ms: 0, pass: true, note: "SKIP" });
      }
    }
  }

  const ok = printResults();
  process.exit(ok ? 0 : 1);
}

main().catch((err) => {
  console.error(RED + "Script crashed:" + RESET, err);
  process.exit(1);
});

// ---------------------------------------------------------------------------
// Extending this for real workflow/mutation testing:
// You'll need real UUIDs from your database (an internship_id, a report_id,
// a faculty user assigned as mentor, etc.) since these routes 404/error on
// the zero-UUID placeholders used above. Example:
//
//   const res = await timedFetch("POST", "/api/coordinator/internships", {
//     auth: true,
//     body: { student_id: "<a real STUDENT-role user id>", company_name: "Acme", ... },
//   });
//
// Also note ACCESS_TOKEN's role matters: /api/coordinator/* needs a
// COORDINATOR or ADMIN token, /api/faculty/* needs FACULTY or ADMIN, etc.
// (see internal/http/router.go RBAC groups). Get a token per role by logging
// in as a user who has been assigned that role.
