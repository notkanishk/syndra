import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { AUDIT_ACTIONS } from "@/lib/audit-vocabulary";

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
    const composed = /^(access_request\.|direct_grant\.)|^welcome_bundle_assigned$/;

    const stale = Object.keys(AUDIT_ACTIONS).filter(
      (action) => !written.has(action) && !composed.test(action),
    );

    expect(stale, "copy for an action nothing emits — either it was renamed or it is dead").toEqual(
      [],
    );
  });
});
