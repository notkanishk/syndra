import { ESLint } from "eslint";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The inline-sentence lint guard for results/*.tsx, proved by mutation: a
 * sentence written in a renderer, however it is spelled, is refused; a class
 * name, an import and a one-word label pass; and the same text is allowed in
 * copy.ts and in a test.
 */
const root = join(import.meta.dirname, "../../..");
const eslint = new ESLint({ cwd: root });
const lint = async (code: string, file: string) => (await eslint.lintText(code, { filePath: join(root, file) }))[0].messages;
const rule = async (code: string, file = "src/components/results/Mutant.tsx") => (await lint(code, file)).filter((m) => m.ruleId === "no-restricted-syntax");

const SENTENCES: Record<string, string> = {
  "multi-line JSX text": "export const a = (\n  <p>\n    Nothing\n    here\n  </p>\n);",
  "JSX text with a value": "export const a = (n: number) => <p>Read {n} places today</p>;",
  "template starting with a value": "export const a = (name: string) => `${name} could not be read.`;",
  "lower-case template fragment": "export const a = (limit: string) => `of ${limit}`;",
  "single-quoted sentence": "export const a = 'Some usage was missing.';",
  "sentence with a digit and a question mark": 'export const a = "Is 1 place left?";',
  "JSX text before a value": "export const a = (n: number) => <p>Read {n}</p>;",
  "JSX text after a value": "export const a = (n: number) => <p>{n} places</p>;",
  "aria-label text": 'export const a = <span aria-label="Used so far" />;',
  "title text": 'export const a = <span title="Read just now" />;',
};

describe("a sentence in a results renderer is refused", () => {
  for (const [what, code] of Object.entries(SENTENCES)) {
    it(`refuses ${what}`, async () => {
      expect((await rule(code)).length).toBeGreaterThan(0);
    });
  }

  it("passes a class name, data attribute, key, import, directive, one-word label and a join", async () => {
    const ok = [
      '"use client";',
      'import { a } from "@/components/ui/Some thing";',
      'export const a = <b className="flex flex-col gap-3" data-x="a b" key="a b">Refused</b>;',
      "export const b = (x: string, y: string) => `${x} ${y}`;\nexport const d = <b>\n  Refused\n</b>;",
      'export const c = ["a", "b"].join(" · ");',
    ].join("\n");
    expect(await rule(ok)).toEqual([]);
  });

  it("allows the same sentences in copy.ts and in a test", async () => {
    const code = SENTENCES["single-quoted sentence"];
    expect(await rule(code, "src/components/results/copy.ts")).toEqual([]);
    expect(await rule(code, "src/components/results/__tests__/X.test.tsx")).toEqual([]);
  });

  it("names where the sentence goes", async () => {
    expect((await rule(SENTENCES["single-quoted sentence"]))[0].message).toContain("results/copy.ts");
  });

  // addon-contract 6c: the same guard holds the member's actions (copy guide section 14).
  it("refuses a sentence in an actions renderer, and allows it in actions/copy.ts", async () => {
    const code = SENTENCES["single-quoted sentence"];
    expect((await rule(code, "src/components/actions/Mutant.tsx")).length).toBeGreaterThan(0);
    expect((await rule(SENTENCES["aria-label text"], "src/components/actions/Mutant.tsx")).length).toBeGreaterThan(0);
    expect(await rule(code, "src/components/actions/copy.ts")).toEqual([]);
    expect((await rule(code, "src/components/actions/Mutant.tsx"))[0].message).toContain("actions/copy.ts");
  });
});
