import { ESLint } from "eslint";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The storage-word lint guard (addon-contract 3.10g), proved by mutation: the
 * storage kind's words are refused in the UI core and allowed in the storage
 * module, and re-cutting the eslint blocks has not stopped the other two
 * guards (invalidation, field names) from applying where they should.
 */
const root = join(import.meta.dirname, "../../..");
const eslint = new ESLint({ cwd: root });
const lint = async (code: string, file: string) => (await eslint.lintText(code, { filePath: join(root, file) }))[0].messages.filter((m) => m.ruleId === "no-restricted-syntax");

const WORDS = [
  'export const a = "Network storage";',
  "export const a = <p>Your folders</p>;",
  "export const a = (x: number) => `${x} shares`;",
  'export const a = "SMB";',
  'export const a = "mounted";',
  'export const a = "Mount";',
  "export const a = `Shares`;",
];
const CORE = ["src/components/results/Mutant.tsx", "src/components/targets/Mutant.tsx", "src/components/people/Mutant.tsx", "src/lib/queries/useThing.tsx", "src/lib/fields.ts"];
const HOMES = ["src/components/kinds/storage/Mutant.tsx", "src/app/storage/Mutant.tsx", "src/lib/nav.ts"];

// JSX cannot parse in a .ts file, and a parse error is no rule message: it would pass an "allowed" check for nothing.
const where = (code: string, files: string[]) => files.filter((f) => !code.includes("<p>") || f.endsWith(".tsx"));

describe("the storage kind's words are written only in its own module", () => {
  for (const code of WORDS) {
    it(`refuses ${code}`, async () => {
      for (const file of where(code, CORE)) expect((await lint(code, file)).length, file).toBeGreaterThan(0);
    });
    it(`allows it in the storage kind, app/storage and nav.ts: ${code}`, async () => {
      for (const file of where(code, HOMES)) expect(await lint(code, file), file).toEqual([]);
    });
    it(`allows it in a test: ${code}`, async () => {
      expect(await lint(code, "src/components/targets/__tests__/X.test.tsx")).toEqual([]);
    });
  }

  it("leaves other words and word-pieces alone", async () => {
    const ok = 'export const a = ["sharing", "Folderol", "unmounted", "SMBs", "Network"];';
    expect(await lint(ok, "src/components/targets/Mutant.tsx")).toEqual([]);
  });
});

describe("re-cutting the blocks left the other guards in force", () => {
  const invalidate = "export const a = (q: { invalidateQueries: () => void }) => q.invalidateQueries();";
  const field = 'export const a = "smb_enabled";';

  it("still refuses a hand-made invalidation in the core, the storage kind, app/storage, nav.ts and fields.ts", async () => {
    for (const file of ["src/components/targets/Mutant.tsx", ...HOMES, "src/lib/fields.ts"]) {
      expect((await lint(invalidate, file)).length, file).toBeGreaterThan(0);
    }
  });

  it("still refuses a field name in the core, app/storage and nav.ts, and allows it in the storage kind and fields.ts", async () => {
    for (const file of ["src/components/targets/Mutant.tsx", "src/app/storage/Mutant.tsx", "src/lib/nav.ts", "src/lib/queries/invalidation.ts"]) {
      expect((await lint(field, file)).length, file).toBeGreaterThan(0);
    }
    for (const file of ["src/components/kinds/storage/Mutant.tsx", "src/lib/fields.ts"]) {
      expect(await lint(field, file), file).toEqual([]);
    }
  });

  it("still refuses a storage word in invalidation.ts, which owns the invalidation map", async () => {
    expect((await lint('export const a = "Network storage";', "src/lib/queries/invalidation.ts")).length).toBeGreaterThan(0);
  });

  it("still allows invalidateQueries in invalidation.ts itself", async () => {
    expect(await lint(invalidate, "src/lib/queries/invalidation.ts")).toEqual([]);
  });
});
