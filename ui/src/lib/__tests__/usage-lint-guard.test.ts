import { ESLint } from "eslint";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The statistics renderers write no sentence of their own (copy guide section
 * 15), proved by mutation: an inline sentence is refused in components/usage/*.tsx
 * and allowed in its copy.ts, a class name is not a sentence, and the kind-word
 * guard still applies there.
 */
const root = join(import.meta.dirname, "../../..");
const eslint = new ESLint({ cwd: root });
const lint = async (code: string, file: string) => (await eslint.lintText(code, { filePath: join(root, file) }))[0].messages.filter((m) => m.ruleId === "no-restricted-syntax");

const SENTENCES = [
  'export const a = <p>Nothing counted here</p>;',
  'export const a = "Nothing counted here";',
  "export const a = (n: number) => `${n} in the last 30 days`;",
  'export const a = <svg aria-label="Days that were not counted" />;',
];

describe("no sentence in a statistics renderer", () => {
  for (const code of SENTENCES) {
    it(`refuses ${code}`, async () => {
      expect((await lint(code, "src/components/usage/Mutant.tsx")).length).toBeGreaterThan(0);
    });
  }
  it("allows them in copy.ts, and a class name is not a sentence", async () => {
    expect(await lint('export const a = "Nothing counted here";', "src/components/usage/copy.ts")).toEqual([]);
    expect(await lint('export const a = <p className="flex flex-col gap-2" />;', "src/components/usage/Mutant.tsx")).toEqual([]);
  });
  it("still refuses the storage kind's words there", async () => {
    expect((await lint('export const a = "SMB";', "src/components/usage/Mutant.tsx")).length).toBeGreaterThan(0);
  });
});
