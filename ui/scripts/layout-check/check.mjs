/**
 * The layout check: a real browser, every page, every width, both views.
 *
 * jsdom lays nothing out, which is how a column starved to 36px and a People
 * count cut to "2 u" reached staging past a green suite (audit, 19 Nov 2026).
 * This loads each page in Chrome at each width, runs `detect.js` in it, and
 * exits non-zero on any overlap over 2px, any clipped text outside a
 * `truncate`, any column heading more than 4px off its cells, a seat off its
 * row's first line when the row fits, or a page that scrolls sideways.
 *
 *   bun run layout:check -- --base http://192.168.5.16:3001
 *
 * Options (all optional but --base):
 *   --widths 390,768,1024,1100,1280,1440   1100 is in the band (1080–1240) only it catches
 *   --pages /,/users,...                   default: every list page below, plus detail pages
 *                                          found from the lists (first rows of People, Projects, Bundles)
 *   --views basic,advanced
 *   --themes light                         light and dark never differed in the audit
 *   --user dev_admin                       the demo identity it signs in as
 *   --record file.json / --replay file.json
 *       Record the API's answers while checking one server, and replay them into
 *       another: check a local build (`bun next dev`, no ZITADEL_DOMAIN) against
 *       staging's data before it is deployed. The file holds real names: keep it
 *       outside the repo.
 *
 * Read-only: it signs in, reads pages, and never clicks.
 * System Chrome (`channel: "chrome"`); CHROME_PATH overrides it.
 * Exit codes, as `sweep:touch`: 1 found something, 2 signed out (wrong page), 3 could not run.
 */
import { readFileSync, writeFileSync } from "node:fs";
import { parseArgs } from "node:util";

import { chromium } from "playwright-core";

import { detect } from "./detect.js";

const PAGES = [
  "/", "/users", "/projects", "/roles", "/applications", "/bundles", "/policies", "/requests",
  "/governance/pending", "/operations", "/operations/cascades", "/changes/roles-that-wait",
  "/governance/drift", "/governance/unconfirmed-revocations", "/review/expiring-access",
  "/review/holds", "/audit", "/system/targets", "/system/targets/truenas", "/system/service-accounts",
  "/insights", "/insights?tab=trends", "/insights?tab=usage", "/settings", "/zitadel",
  "/zitadel/grants", "/zitadel/projects",
];
const DISCOVER = ["/users", "/projects", "/bundles", "/applications"];

const { values: opt } = parseArgs({
  options: {
    base: { type: "string" },
    widths: { type: "string", default: "390,768,1024,1100,1280,1440" },
    pages: { type: "string" },
    views: { type: "string", default: "basic,advanced" },
    themes: { type: "string", default: "light" },
    user: { type: "string", default: "dev_admin" },
    record: { type: "string" },
    replay: { type: "string" },
  },
});
if (!opt.base) {
  console.error("Usage: bun run layout:check -- --base http://192.168.5.16:3001 [--widths …] [--pages …]");
  process.exit(3);
}
const base = opt.base.replace(/\/$/, "");
const list = (s) => s.split(",").map((x) => x.trim()).filter(Boolean);
const widths = list(opt.widths).map(Number);

const tape = opt.replay ? JSON.parse(readFileSync(opt.replay, "utf8")) : {};
const keyOf = (req) => `${req.method()} ${new URL(req.url()).pathname}${new URL(req.url()).search} ${req.postData() ?? ""}`;

let browser;
try {
  browser = await chromium.launch(process.env.CHROME_PATH ? { executablePath: process.env.CHROME_PATH } : { channel: "chrome" });
} catch (error) {
  console.error(`Could not start Chrome: ${error.message}`);
  process.exit(3);
}

async function context(view) {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  if (opt.replay) {
    await ctx.route("**/api/proxy/**", (route) => {
      const hit = tape[keyOf(route.request())];
      return hit
        ? route.fulfill({ status: hit.status, contentType: hit.type, body: hit.body })
        : route.fulfill({ status: 404, contentType: "application/json", body: "{}" });
    });
  }
  if (opt.record) {
    ctx.on("response", async (res) => {
      if (!res.url().includes("/api/proxy/")) return;
      try {
        tape[keyOf(res.request())] = { status: res.status(), type: res.headers()["content-type"] ?? "application/json", body: await res.text() };
      } catch {}
    });
  }
  const page = await ctx.newPage();
  await page.goto(`${base}/login`);
  await page.evaluate(
    (user) => fetch("/auth/login", { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: `userId=${user}&next=/` }),
    opt.user,
  );
  await ctx.addCookies([{ name: "syndra_ui_view", value: view, url: base }]);
  await page.goto(`${base}/`);
  if (new URL(page.url()).pathname === "/login") {
    console.error(`Not signed in as ${opt.user}: ${base} has no demo sign-in for it.`);
    process.exit(2);
  }
  await page.close();
  return ctx;
}

// Until the DOM has been quiet for 1.2s (at most 20s): lists arrive, names resolve, rows settle.
const settle = (page) =>
  page.evaluate(async () => {
    let last = Date.now();
    const mo = new MutationObserver(() => (last = Date.now()));
    mo.observe(document, { subtree: true, childList: true, characterData: true, attributes: true });
    const t0 = Date.now();
    while (Date.now() - t0 < 20000 && (Date.now() - last < 1200 || document.readyState !== "complete")) {
      await new Promise((r) => setTimeout(r, 200));
    }
    mo.disconnect();
  });

async function discover(ctx) {
  const page = await ctx.newPage();
  const found = [];
  for (const path of DISCOVER) {
    await page.goto(base + path);
    await settle(page);
    const hrefs = await page.$$eval(`a[href^="${path}/"]`, (as) => as.map((a) => a.getAttribute("href")));
    found.push(...[...new Set(hrefs)].filter((h) => /\/[^/]+\/[^/?#]+$/.test(h)).slice(0, 2));
  }
  await page.close();
  return found;
}

const findings = [];
let runs = 0;
for (const view of list(opt.views)) {
  const ctx = await context(view);
  const pages = opt.pages ? list(opt.pages) : [...PAGES, ...(await discover(ctx))];
  for (const path of [...new Set(pages)]) {
    await Promise.all(
      widths.map(async (width) => {
        const page = await ctx.newPage();
        await page.setViewportSize({ width, height: 900 });
        // A page that never answers is a finding, not a hung check (evaluate has no timeout of its own).
        const stuck = setTimeout(() => page.close().catch(() => {}), 90_000);
        try {
          await page.goto(base + path, { waitUntil: "load" });
          if (new URL(page.url()).pathname === "/login") {
            console.error(`${path}: bounced to /login, the session was lost.`);
            process.exit(2);
          }
          await settle(page);
          for (const theme of list(opt.themes)) {
            await page.evaluate((t) => document.documentElement.setAttribute("data-theme", t), theme);
            await page.waitForTimeout(150);
            const found = await page.evaluate(detect, { tablet: 720 });
            runs++;
            for (const [kind, lines] of Object.entries(found)) {
              for (const line of lines) findings.push(`${path} ${width} ${view} ${theme} ${kind} ${line}`);
            }
          }
        } catch (error) {
          findings.push(`${path} ${width} ${view} error ${error.message.split("\n")[0]}`);
        } finally {
          clearTimeout(stuck);
          await page.close().catch(() => {});
        }
      }),
    );
    console.error(`checked ${path} (${view})`);
  }
  await ctx.close();
}
await browser.close();
if (opt.record) writeFileSync(opt.record, JSON.stringify(tape));

const unique = [...new Set(findings)];
for (const line of unique) console.log(line);
console.error(`${runs} page runs, ${unique.length} findings.`);
process.exit(unique.length ? 1 : 0);
