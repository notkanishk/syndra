import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { MEMBER, whyLine, type Reader } from "@/components/results/copy";
import { UNCONFIRMED_NEXT, UNCONFIRMED_WORD, NOTHING_SENT_WORD } from "@/lib/outcome";

/**
 * The copy guide's section 19 and `whyLine` say the same words (design §13.1). The backend's
 * `whySentence` is held to the same table by its own test, so the two ends never drift apart.
 */
const guide = readFileSync(resolve(__dirname, "../../../../openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 19. When a system is not answering"));

const rows = section
  .split("\n")
  .filter((line) => /^\| (operator|member read|member write) \|/.test(line))
  .map((line) => line.split("|").slice(1, -1).map((cell) => cell.trim()))
  .map(([reader, reasons, cause, wait, says]) => ({ reader, reasons: reasons.split(", "), cause, wait, says }));

const OPERATOR: Reader = { audience: "operator", person: "" };
const fill = (s: string) => s.replaceAll("{system}", "Ringside NAS").replaceAll("{product}", "TrueNAS").replaceAll("{n}", "9");

describe("why a system was not read, as the guide's section 19 says it", () => {
  it("finds the table", () => {
    expect(rows.length).toBeGreaterThan(10);
  });

  // The UI composes the read sentences; a member's write sentence and `unusable` only come inside a 503's message.
  const said = rows.filter((r) => r.reader !== "member write" && r.cause !== "unusable");
  it.each(said.flatMap((r) => r.reasons.map((reason) => [r.reader, reason, r.cause, r] as const)))("%s, %s, %s", (reader, reason, cause, row) => {
    const why = { reason, ...(cause !== "—" ? { cause } : {}), ...(row.wait !== "—" ? { retryAfter: 9 } : {}) };
    expect(whyLine(why, "Ringside NAS", reader === "operator" ? OPERATOR : MEMBER, "TrueNAS")).toBe(fill(row.says));
  });

  it("says the add-on's own sentence first where it is the one that knows", () => {
    const detail = "TrueNAS is restarting its middleware";
    expect(whyLine({ reason: "app_unreachable", detail }, "Ringside NAS", OPERATOR)).toBe(`${detail}.`);
    expect(whyLine({ reason: "addon_unreachable", detail }, "Ringside NAS", OPERATOR)).not.toContain(detail);
  });

  it("says a 503's own sentence as it came", () => {
    expect(whyLine({ reason: "addon_unreachable", message: "Not saved. Syndra couldn't check this value on Ringside NAS. Try again when it answers." }, "Ringside NAS", OPERATOR)).toMatch(/^Not saved\./);
  });

  it("words the write outcomes as the second table does", () => {
    expect(section).toContain(`| ${UNCONFIRMED_WORD} · This may have been applied. · ${UNCONFIRMED_NEXT} |`);
    expect(section).toContain(`| ${NOTHING_SENT_WORD} |`);
  });
});
