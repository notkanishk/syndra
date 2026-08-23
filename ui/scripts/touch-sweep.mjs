/**
 * The check jsdom cannot make.
 *
 * Three classes of defect on this product are invisible to every static guard
 * in the suite, and all three shipped on this branch: a page that could not
 * scroll (one missing `min-h-0`), controls under the 44px floor that no
 * component owns (raw links in a row of links), and a tab bar pushed below the
 * fold. They need a browser with layout, so this is a script rather than a
 * test.
 *
 * The inline-link case is why this exists instead of a lint rule. WCAG 2.5.8
 * exempts a target inside a sentence, and whether a link is in a sentence is
 * not decidable from the source: the roles page's "Check a project's roles
 * upstream" and the Makerspace footer's "17 roles nobody holds" are the same
 * JSX shape, and only the first is exempt. The rule is therefore applied here
 * against rendered prose — a sub-floor target is reported unless its own
 * parent holds at least 40 characters of non-target text.
 *
 * No dependencies: it drives a Chrome that is already running, over CDP.
 *
 *   1. chrome --remote-debugging-port=9222
 *   2. bun next dev -p 3010      (no ZITADEL_DOMAIN, so demo sign-in works)
 *   3. bun run sweep:touch
 *
 * BASE, CDP and DETAIL_ROUTES override the defaults. Exits non-zero on any
 * finding, so it can gate a release even though it cannot gate a commit — it
 * needs a reachable backend, and an empty list hides most of what it looks
 * for.
 */

const BASE = process.env.BASE ?? "http://localhost:3010";
const CDP = process.env.CDP ?? "http://localhost:9222";
// 44 until the desktop breakpoint, not past it: the product releases the floor
// above 1080 deliberately (`desktop:min-h-0` on every control), where it is a
// dense rail-and-table application driven by a pointer. WCAG 2.5.8's 24px
// still applies there.
const TOUCH_FLOOR = 44;
const POINTER_FLOOR = 24;
const PROSE_CHARS = 40;

// Every destination in nav.ts, the 404, and the dynamic routes. Detail routes
// are listed explicitly because they are NOT in nav.ts — which is exactly why
// the first sweep of this branch missed an 18px breadcrumb present on all of
// them. Override with real ids from your own data.
const ROUTES = [
  "/", "/applications", "/audit", "/automation/settings", "/bundles",
  "/governance/drift", "/governance/pending", "/governance/unconfirmed-revocations",
  "/graph", "/operations", "/operations/cascades", "/policies", "/projects",
  "/requests", "/review/expiring-access", "/review/holds", "/roles", "/storage",
  "/users", "/zitadel", "/no-such-page",
  ...(process.env.DETAIL_ROUTES ?? "").split(",").filter(Boolean),
];

const VIEWPORTS = [
  { name: "phone", width: 390, height: 844, scale: 3, mobile: true, floor: TOUCH_FLOOR },
  { name: "tablet", width: 744, height: 1133, scale: 2, mobile: false, floor: TOUCH_FLOOR },
  { name: "desktop", width: 1280, height: 900, scale: 2, mobile: false, floor: POINTER_FLOOR },
];

/** Runs in the page. Everything this branch learned to look for, in one pass. */
const auditFor = (floor) => `(() => {
  const FLOOR = ${floor}, PROSE = ${PROSE_CHARS};
  const issues = [];
  const scroll = document.getElementById("app-scroll");

  if (document.documentElement.scrollWidth > window.innerWidth) {
    issues.push("horizontal overflow by " + (document.documentElement.scrollWidth - window.innerWidth) + "px");
  }

  if (scroll) {
    scroll.scrollTop = scroll.scrollHeight;
    if (scroll.scrollHeight > scroll.clientHeight && scroll.scrollTop === 0) {
      issues.push("cannot scroll (" + scroll.scrollHeight + "px of content in " + scroll.clientHeight + "px)");
    }
    scroll.scrollTop = 0;
  }

  const bar = Array.from(document.querySelectorAll('nav[aria-label="Primary"]'))
    .find((n) => n.getBoundingClientRect().height > 0 && String(n.className).includes("tablet:hidden"));
  if (bar && Math.round(bar.getBoundingClientRect().bottom) !== window.innerHeight) {
    issues.push("tab bar bottom at " + Math.round(bar.getBoundingClientRect().bottom) + ", viewport " + window.innerHeight);
  }

  for (const el of document.querySelectorAll("a[href], button, input, select, [role='button']")) {
    const rect = el.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0 || rect.height >= FLOOR) continue;
    const own = (el.textContent || "").trim();
    // Next's dev overlay and the query devtools are not the product, and they
    // are only present in dev. Matched on the accessible name, because both
    // are icon buttons with no text content.
    const name = el.getAttribute("aria-label") || own;
    if (/Tanstack|Next\.js|devtools/i.test(name)) continue;
    const parent = (el.parentElement ? el.parentElement.textContent : "" || "").trim();
    const prose = parent.replace(own, "").replace(/[·/|,\\s]+/g, " ").trim();
    if (prose.length >= PROSE) continue;
    issues.push(Math.round(rect.height) + "px target \\"" + name.slice(0, 40) + "\\"");
  }

  for (const el of document.querySelectorAll("body *")) {
    if (el.children.length || el.closest("[aria-hidden='true']")) continue;
    if (!el.textContent || !el.textContent.trim()) continue;
    const size = parseFloat(getComputedStyle(el).fontSize);
    if (size < 12.5) issues.push(size + "px text \\"" + el.textContent.trim().slice(0, 30) + "\\"");
  }

  return JSON.stringify(issues);
})()`;

const targets = await (await fetch(`${CDP}/json/new`, { method: "PUT" })).json();

const socket = new WebSocket(targets.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  socket.onopen = resolve;
  socket.onerror = () => reject(new Error(`No Chrome at ${CDP}. Start one with --remote-debugging-port=9222.`));
});

let nextId = 0;
const pending = new Map();
socket.onmessage = (event) => {
  const message = JSON.parse(event.data);
  if (message.id && pending.has(message.id)) {
    pending.get(message.id)(message);
    pending.delete(message.id);
  }
};

function send(method, params = {}) {
  const id = ++nextId;
  return new Promise((resolve) => {
    pending.set(id, resolve);
    socket.send(JSON.stringify({ id, method, params }));
  });
}

const evaluate = async (expression) => {
  const reply = await send("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
  return reply.result?.result?.value;
};

const settle = (ms) => new Promise((r) => setTimeout(r, ms));

await send("Page.enable");

// Navigate first. `/json/new` opens about:blank whatever URL it is handed, and
// a relative fetch from there has no base to resolve against — which threw,
// left the browser signed out, and made every route below redirect to /login.
// The sweep then reported that page as clean, 75 times. Hence the assertion
// underneath: a check that cannot tell it is looking at the wrong page is
// worse than no check.
await send("Page.navigate", { url: BASE + "/login" });
await settle(1200);
await evaluate(`fetch("/auth/login", { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: "userId=dev_admin&next=/" })`);
await settle(800);

await send("Page.navigate", { url: BASE + "/" });
await settle(1500);
if ((await evaluate("location.pathname")) === "/login") {
  console.error("Not signed in. The dev server must run without ZITADEL_DOMAIN and with SESSION_SECRET set.");
  process.exit(2);
}

let failures = 0;
for (const viewport of VIEWPORTS) {
  await send("Emulation.setDeviceMetricsOverride", {
    width: viewport.width,
    height: viewport.height,
    deviceScaleFactor: viewport.scale,
    mobile: viewport.mobile,
  });
  for (const route of ROUTES) {
    await send("Page.navigate", { url: BASE + route });
    await settle(1500);
    const landed = await evaluate("location.pathname");
    if (landed === "/login" && route !== "/login") {
      console.error(`${viewport.name} ${route}: bounced to /login — the session was lost mid-sweep.`);
      process.exit(2);
    }
    // The shell, or this is not a page of the product. An error page has no
    // scroller and almost no controls, so it audits clean — which is how a dev
    // server broken by a concurrent `next build` (they share `.next/`) once
    // produced a green sweep across every route. Silence has to be earned.
    if (!(await evaluate('!!document.getElementById("app-scroll")'))) {
      console.error(`${viewport.name} ${route}: no app shell rendered — the page failed to load.`);
      process.exit(2);
    }
    const issues = JSON.parse((await evaluate(auditFor(viewport.floor))) ?? "[]");
    if (issues.length) {
      failures += issues.length;
      console.log(`${viewport.name} ${route}`);
      for (const issue of issues) console.log(`  ${issue}`);
    }
  }
}

socket.close();
console.log(failures === 0 ? "clean" : `${failures} finding(s)`);
process.exit(failures === 0 ? 0 : 1);
