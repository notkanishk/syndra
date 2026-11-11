import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { AUDIT_ACTIONS, describeAction } from "@/lib/audit-vocabulary";
import { targetLabel } from "@/lib/nav";

/**
 * The action vocabulary falls through to the raw key for anything it does not recognise, which is
 * the right failure mode — a log that invents a description is worse than one that admits it does
 * not know — but it is a SILENT one. Six actions accumulated behind it before anyone noticed:
 * `bundle.updated`, `bundle.deleted`, `bundle.version_published`, `bundle.holder_moved`,
 * `mapping_rule.deleted` and `access_request.withdrawn`, all rendering as machine keys on a page
 * whose entire purpose is to be readable.
 *
 * So the map is checked against the backend rather than against a list somebody has to remember to
 * update. This reads the Go sources for any dotted literal on a line that either names an `Action:`
 * field or mentions audit at all — which covers `CascadeAudit{Action: …}`, `InsertAuditLog(…)` and
 * helpers like `auditClaimShape(…)` — and fails naming any action with no sentence.
 *
 * It cannot see the two actions built by concatenation (`"access_request."+status`,
 * `"direct_grant."+opTypeAuditVerb(...)`). Both families are already covered, and a scanner that
 * evaluated Go string arithmetic would be a worse thing to own than this comment.
 */
const BACKEND = resolve(process.cwd(), "..", "backend", "internal");

function goFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) return goFiles(path);
    return entry.endsWith(".go") && !entry.endsWith("_test.go") ? [path] : [];
  });
}

function auditActionsWrittenByTheBackend(): Map<string, string> {
  const found = new Map<string, string>();
  for (const file of goFiles(BACKEND)) {
    const lines = readFileSync(file, "utf8").split("\n");
    lines.forEach((line, index) => {
      if (!line.includes("Action:") && !/[Aa]udit/.test(line)) return;
      for (const match of line.matchAll(/"([a-z_]+\.[a-z_]+)"/g)) {
        // A Go filename in an error string would match the shape; nothing else does.
        if (match[1].endsWith(".go")) continue;
        found.set(match[1], `${file.slice(BACKEND.length + 1)}:${index + 1}`);
      }
    });
  }
  return found;
}

/**
 * The actions built by joining strings with a system's name in the middle: the
 * scanner above cannot see them (no dotted literal), and two went unworded
 * because of it. Read from any line, for the two families that carry a system
 * (`entitlement`, `allowance`), in the three ways the backend joins one:
 * Go `"entitlement."+target+".verb"`, SQL `'allowance.' || l.target || '.verb'`
 * and `fmt.Sprintf("entitlement.%s.verb", …)`. Returned as the action a real
 * system would write, so each can be put to `describeAction`.
 */
export function joinedActionsIn(source: string): string[] {
  const join = String.raw`["']\s*(?:\+|\|\|)\s*[\w.()]+\s*(?:\+|\|\|)\s*["']\.|%s\.`;
  const re = new RegExp(String.raw`["'](entitlement|allowance)\.(?:${join})([a-z_]+)["']`, "g");
  return [...source.matchAll(re)].map((m) => `${m[1]}.truenas.${m[2]}`);
}

/** The ones with no sentence: describeAction falls back to the raw key for them. */
const unworded = (actions: string[]) => actions.filter((a) => describeAction(a, targetLabel).verb === a);

describe("joined audit actions (entitlement.<system>.*)", () => {
  it("has a sentence for every one the backend writes", () => {
    const joined = new Map<string, string>();
    for (const file of goFiles(BACKEND)) {
      for (const a of joinedActionsIn(readFileSync(file, "utf8"))) joined.set(a, file.slice(BACKEND.length + 1));
    }
    // A floor per family, each its real count today: entitlement enqueued, superseded and abandoned;
    // allowance lapsed (the SQL join). If the scanner stops seeing a form, its family drops under it.
    const family = (f: string) => [...joined.keys()].filter((a) => a.startsWith(`${f}.`)).length;
    expect(family("entitlement")).toBeGreaterThanOrEqual(3);
    expect(family("allowance")).toBeGreaterThanOrEqual(1);
    expect(unworded([...joined.keys()]), "these render as raw machine keys in Audit and Person Activity").toEqual([]);
  });

  // Mutation check: the guard must fail on a verb with no wording, on a system name that is not
  // the first one tried, and must not be fooled by a pattern that only matches `truenas`.
  it("fails on a joined action nobody worded", () => {
    const mutant = 'tx.Exec(ctx, q, target, "entitlement."+target+".vanished")';
    expect(joinedActionsIn(mutant)).toEqual(["entitlement.truenas.vanished"]);
    expect(unworded(joinedActionsIn(mutant))).toEqual(["entitlement.truenas.vanished"]);
  });

  it("sees the SQL and Sprintf joins as well as the Go one", () => {
    expect(joinedActionsIn("SELECT 'system', l.subject_id, 'allowance.' || l.target || '.lapsed', l.id")).toEqual(["allowance.truenas.lapsed"]);
    expect(joinedActionsIn('fmt.Sprintf("entitlement.%s.vanished", target)')).toEqual(["entitlement.truenas.vanished"]);
    expect(unworded(joinedActionsIn("'allowance.' || t || '.vanished'"))).toEqual(["allowance.truenas.vanished"]);
  });

  it("words them for any system, in plain language", () => {
    for (const verb of ["superseded", "abandoned"]) {
      const said = describeAction(`entitlement.unifi-doors.${verb}`, targetLabel).verb;
      expect(said).not.toContain("entitlement.");
      expect(said).toMatch(/queued change/);
    }
    expect(describeAction("entitlement.unifi-doors.abandoned", targetLabel).verb).toContain("was disconnected from Syndra before it was sent");
  });
});

describe("audit action vocabulary", () => {
  it("has a sentence for every action the backend writes", () => {
    const written = auditActionsWrittenByTheBackend();

    // If this is zero the scanner broke, and a passing test would mean nothing.
    expect(written.size).toBeGreaterThan(10);

    const missing = [...written.entries()]
      .filter(([action]) => !(action in AUDIT_ACTIONS))
      .map(([action, where]) => `${action} (${where})`);

    expect(missing, "these render as raw machine keys in Audit and Person Activity").toEqual([]);
  });

  it("names no action the backend never writes", () => {
    const written = auditActionsWrittenByTheBackend();
    // The two concatenated families, and the one action written by the onboarding path under a
    // name with no dot in it.
    // Plus the two the backend stopped writing when Retire began keeping its history (19 Oct
    // 2026): rows already in the log still carry them. Likewise the two per-project override
    // actions, retired when each app got its own shape (A-14, 3 Nov 2026).
    const composed =
      /^(access_request\.|direct_grant\.)|^welcome_bundle_assigned$|^(bundle|mapping_rule)\.deleted$|^app_claim_override\.(updated|deleted)$/;

    const stale = Object.keys(AUDIT_ACTIONS).filter(
      (action) => !written.has(action) && !composed.test(action),
    );

    expect(stale, "copy for an action nothing emits — either it was renamed or it is dead").toEqual(
      [],
    );
  });
});
