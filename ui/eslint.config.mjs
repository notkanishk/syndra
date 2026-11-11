import { dirname } from "path";
import { fileURLToPath } from "url";
import { FlatCompat } from "@eslint/eslintrc";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({
  baseDirectory: __dirname,
});

const eslintConfig = [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    rules: {
      /**
       * An error, not a warning.
       *
       * Three of these sat in the output as warnings and two of them were
       * live defects, not tidiness: `DeleteBundleDialog` and the rule-delete
       * dialog each took an `onDeleted` callback and never called it, so
       * deleting left the parent selecting a bundle that no longer existed
       * and an editor open on a rule that no longer existed. The linter had
       * found both and nobody had to look, because a warning is a thing a
       * clean run still prints.
       *
       * A declared prop that is never referenced is a wire that was never
       * connected. Deliberately-unused bindings say so with a leading
       * underscore.
       */
      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          destructuredArrayIgnorePattern: "^_",
        },
      ],
    },
  },
  ...(() => {
    const message = (m) => ({ message: m });
    /**
     * One invalidation map (one-path W2c). A write learns which reads it made
     * stale in exactly one place, `invalidateAfter` in
     * src/lib/queries/invalidation.ts; nothing else names a key to
     * invalidate. Matched on the syntax tree, so `x.invalidateQueries`,
     * `x["invalidateQueries"]` and destructuring it out all fail. Tests may
     * spy on it.
     */
    const invalidation = [
      "MemberExpression[property.name='invalidateQueries']",
      "MemberExpression[computed=true][property.value='invalidateQueries']",
      "ObjectPattern > Property[key.name='invalidateQueries']",
    ].map((selector) => ({
      selector,
      ...message("Invalidate through invalidateAfter(qc, kind, scope) in lib/queries/invalidation.ts, the one map."),
    }));
    /**
     * The UI reads fields by role (addon-contract 3.4). The five TrueNAS field
     * names and `smb` are written only in lib/fields.ts and in
     * components/kinds/storage/; everywhere else a screen asks fieldsByRole /
     * useFieldSchema and says what comes back in the add-on's label. "group"
     * is also a layout word (a nav group, role="group"), so it is refused only
     * where it names a field: compared with `.field`, a `field=` prop, a
     * `field:` property, or a FIELD_* constant.
     */
    const fieldWord = "Literal[value=/^(home|home_quota|enabled|smb_enabled|smb)$/]";
    const roles = [
      fieldWord,
      "BinaryExpression:has(> MemberExpression[property.name='field']) > Literal[value='group']",
      "JSXAttribute[name.name='field'] > Literal[value='group']",
      "Property[key.name='field'] > Literal[value='group']",
      "VariableDeclarator[id.name=/^FIELD/] > Literal[value='group']",
    ].map((selector) => ({
      selector,
      ...message("Read fields by role through lib/fields.ts (fieldsByRole, useFieldSchema); field names live only in lib/fields.ts and components/kinds/storage/."),
    }));
    const tests = ["src/**/__tests__/**", "src/**/*.test.{ts,tsx}"];
    const storage = ["src/lib/fields.ts", "src/components/kinds/storage/**"];
    return [
      {
        files: ["src/**/*.{ts,tsx}"],
        ignores: ["src/lib/queries/invalidation.ts", ...storage, ...tests],
        rules: { "no-restricted-syntax": ["error", ...invalidation, ...roles] },
      },
      {
        files: ["src/lib/queries/invalidation.ts"],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...roles] },
      },
      {
        files: storage.map((f) => (f.endsWith("**") ? `${f}/*.{ts,tsx}` : f)),
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...invalidation] },
      },
    ];
  })(),
  {
    /**
     * One path for a missing name (one-path W2d). "Unknown project" is said by
     * the names components only: projectLabel(name) where a string is needed,
     * <ProjectName> where a component is. Importing the phrase elsewhere is how
     * a second fallback starts. Its own rule, not no-restricted-syntax: a later
     * block setting that rule would replace the invalidation guard above.
     */
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/components/names/**", "src/**/__tests__/**", "src/**/*.test.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          // By pattern, so the alias and any relative path are both caught.
          patterns: [
            {
              group: ["**/components/names", "**/components/names/Unresolved", "**/names/Unresolved"],
              importNames: ["UNKNOWN_PROJECT"],
              message: "Say a missing project name through projectLabel(name) or <ProjectName>, the one fallback.",
            },
          ],
        },
      ],
    },
  },
];

export default eslintConfig;
