import { ESLint } from "eslint";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * The lint guard (addon-contract 3.4), proved by mutation: the same source
 * that is refused outside the storage kind passes inside it, and the
 * invalidation guard it shares a rule with still bites.
 */
const eslint = new ESLint({ cwd: join(import.meta.dirname, "../../..") });
const lint = async (code: string, file: string) => (await eslint.lintText(code, { filePath: join(import.meta.dirname, "../../..", file) }))[0].messages.filter((m) => m.ruleId === "no-restricted-syntax");

const MUTANTS = [
  'export const a = "smb_enabled";',
  'export const a = "home_quota";',
  'export const a = "enabled";',
  'export const a = "home";',
  'export const a = "smb";',
  'export const a = (x: { field: string }) => x.field === "group";',
  'export const a = <Thing field="group" />;',
  'export const FIELD_X = "group";',
  // The bypasses: a template with no expressions, a switch on the field, a bare `field` identifier.
  "export const a = `smb_enabled`;",
  "export const a = (x: { field: string }) => x.field === `group`;",
  'export const a = (x: { field: string }) => { switch (x.field) { case "group": return 1; } return 0; };',
  'export const a = (field: string) => { switch (field) { case "group": return 1; } return 0; };',
  'export const a = (field: string) => field === "group";',
  'export const a = (field: string) => "group" !== field;',
];

const lintImports = async (code: string, file: string) =>
  (await eslint.lintText(code, { filePath: join(import.meta.dirname, "../../..", file) }))[0].messages.filter((m) => m.ruleId === "no-restricted-imports");

describe("production code does not import fixtures", () => {
  const code = 'import { TRUENAS_SCHEMA } from "@/lib/__fixtures__/truenas-schema";\nexport const a = TRUENAS_SCHEMA;';
  it("refuses it in a screen, a lib file and the names components", async () => {
    for (const file of ["src/components/targets/X.tsx", "src/lib/x.ts", "src/components/names/X.tsx"]) {
      expect((await lintImports(code, file)).length, file).toBeGreaterThan(0);
    }
    expect((await lintImports('import { a } from "../__fixtures__/x";', "src/lib/x.ts")).length).toBeGreaterThan(0);
  });
  it("allows it in a test and in another fixture", async () => {
    expect(await lintImports(code, "src/components/targets/__tests__/X.test.tsx")).toEqual([]);
    expect(await lintImports(code, "src/lib/__fixtures__/other.ts")).toEqual([]);
  });
});

describe("field names are written only in the storage kind and fields.ts", () => {
  for (const code of MUTANTS) {
    it(`refuses ${code}`, async () => {
      expect((await lint(code, "src/components/targets/Mutant.tsx")).length).toBeGreaterThan(0);
      // The generic read renderers know no product and no field.
      expect((await lint(code, "src/components/results/Mutant.tsx")).length).toBeGreaterThan(0);
      // invalidation.ts is plain TS: a JSX mutant cannot be parsed there.
      if (!code.includes("<Thing")) expect((await lint(code, "src/lib/queries/invalidation.ts")).length).toBeGreaterThan(0);
    });
    it(`allows it in the storage kind and in fields.ts: ${code}`, async () => {
      expect(await lint(code, "src/components/kinds/storage/Mutant.tsx")).toEqual([]);
      expect(await lint(code, "src/lib/fields.ts")).toEqual([]);
    });
  }

  it("leaves 'group' alone where it is a layout word", async () => {
    expect(await lint('export const a = <div role="group" />; export const k = { kind: "group" };', "src/components/ui/X.tsx")).toEqual([]);
  });

  it("leaves a template with an expression, and a switch on something else, alone", async () => {
    expect(await lint("export const a = (k: string) => `home ${k}`;", "src/components/ui/X.tsx")).toEqual([]);
    expect(await lint('export const a = (kind: string) => { switch (kind) { case "group": return 1; } return 0; };', "src/components/ui/X.tsx")).toEqual([]);
  });

  it("still refuses a second invalidation path, in the storage kind too", async () => {
    const code = "export const a = (q: { invalidateQueries: () => void }) => q.invalidateQueries();";
    expect((await lint(code, "src/components/targets/X.tsx")).length).toBeGreaterThan(0);
    expect((await lint(code, "src/components/kinds/storage/X.tsx")).length).toBeGreaterThan(0);
  });
});
