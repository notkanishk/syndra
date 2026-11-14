import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { MEMBER, cooldownLine, copyLine, retryingLine, STOP_WAITING, unreachableLine, type Reader } from "@/components/results/copy";
import { COOLDOWN_REFUSED, cooldownNext } from "@/lib/outcome";

/**
 * The copy guide's section 13 and the sentences a card says about how fresh its
 * read is say the same words. If one moves the other fails.
 */
const root = resolve(__dirname, "../../../..");
const guide = readFileSync(resolve(root, "openspec/changes/plain-language-copy/design.md"), "utf8");
const section = guide.slice(guide.indexOf("## 13. How fresh a read is"));

const OPERATOR: Reader = { audience: "operator", person: "{person}" };
const SAID = [
  copyLine("{name}", "{age}"),
  cooldownLine("{name}", 0).replace("0", "{n}"),
  retryingLine("{name}", 0).replace("0", "{n}"),
  unreachableLine("{name}", "{time}", MEMBER),
  unreachableLine("{name}", "{time}", OPERATOR),
  COOLDOWN_REFUSED,
  cooldownNext(1).replace("1", "{n}"),
  cooldownNext(0),
];

describe("the copy guide's section on how fresh a read is", () => {
  it("is in the guide", () => expect(section.length).toBeGreaterThan(0));

  for (const sentence of SAID) {
    it(`carries: ${sentence}`, () => expect(section).toContain(`| ${sentence} |`));
  }

  it("says the control's words, and never Refresh", () => {
    expect(section).toContain("**Read again**");
    expect(section).toContain(`**${STOP_WAITING}**`);
    expect(SAID.join(" ")).not.toMatch(/refresh/i);
  });

  it("is said in plain words (no em dash, no exclamation)", () => {
    for (const s of SAID) expect(s).not.toMatch(/[—!]/);
  });

  // The sentences have one home each: a card that spells one out inline would
  // drift from the guide without this table ever failing.
  it("is written nowhere but the two files that own it", () => {
    const owners = ["src/components/results/copy.ts", "src/lib/outcome.ts"];
    const src = resolve(root, "ui/src");
    const found: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = resolve(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== "__tests__") walk(path);
        } else if (/\.tsx?$/.test(entry.name) && !owners.some((o) => path.endsWith(o))) {
          const code = readFileSync(path, "utf8").replace(/\/\*[\s\S]*?\*\/|^\s*\/\/.*$/gm, "");
          if (/moments ago|read again in \$\{|try again in \$\{/i.test(code)) found.push(path.replace(`${src}/`, ""));
        }
      }
    };
    walk(src);
    expect(found).toEqual([]);
  });
});
