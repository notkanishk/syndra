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
     * `field:` property, a FIELD_* constant, or a `switch` on one. "Compared
     * with a field" is `.field` or a bare `field` identifier; a literal is a
     * string or a template with no expressions (`\`group\``).
     */
    const fieldWord = "Literal[value=/^(home|home_quota|enabled|smb_enabled|smb)$/]";
    const fieldWordTemplate = "TemplateLiteral[expressions.length=0] > TemplateElement[value.cooked=/^(home|home_quota|enabled|smb_enabled|smb)$/]";
    const literalGroup = ["Literal[value='group']", "TemplateLiteral[expressions.length=0] > TemplateElement[value.cooked='group']"];
    const roles = [
      fieldWord,
      fieldWordTemplate,
      ...literalGroup.flatMap((lit) => [
        `BinaryExpression:has(> MemberExpression[property.name='field']) > ${lit}`,
        `BinaryExpression:has(> Identifier[name='field']) > ${lit}`,
        `SwitchStatement[discriminant.property.name='field'] > SwitchCase > ${lit}`,
        `SwitchStatement[discriminant.name='field'] > SwitchCase > ${lit}`,
      ]),
      "JSXAttribute[name.name='field'] > Literal[value='group']",
      "Property[key.name='field'] > Literal[value='group']",
      "VariableDeclarator[id.name=/^FIELD/] > Literal[value='group']",
    ].map((selector) => ({
      selector,
      ...message("Read fields by role through lib/fields.ts (fieldsByRole, useFieldSchema); field names live only in lib/fields.ts and components/kinds/storage/."),
    }));
    /**
     * The UI core does not speak a product's words (addon-contract 3.10g).
     * The storage kind's words (folder, share, SMB, mount, "Network storage")
     * are written only in components/kinds/storage/, app/storage/ and
     * lib/nav.ts. Matched as a string literal, any template element (a
     * sentence is a template with expressions) and JSX text.
     */
    const kindWords = "/\\b([Ff]olders?|[Ss]hares?|SMB|[Mm]ount(ed|s|ing)?|Network storage)\\b/";
    const kindWord = [`Literal[value=${kindWords}]`, `TemplateElement[value.cooked=${kindWords}]`, `JSXText[value=${kindWords}]`].map((selector) => ({
      selector,
      ...message("The storage kind's words live only in components/kinds/storage/ (and app/storage/, lib/nav.ts); say it by role or move it there."),
    }));
    /**
     * No sentence lives in a results renderer. Every sentence of results/*.tsx
     * is written in results/copy.ts, where copy-guide section 12 holds it; the
     * same for actions/*.tsx and actions/copy.ts (section 14).
     * "A sentence" is two words with whitespace between them (any letters), so
     * a multi-line JSX text, one with {values} in it, a template that starts
     * with a value, a lower-case fragment and a single-quoted string are all
     * caught alike. Not human text, so not refused: className/class/key and
     * data-* values, the "use client" directive and import sources. aria-label,
     * title and alt are human text and are NOT exempt.
     */
    // A string: two words. A template element sits beside a value, so one word touching whitespace is enough
    // (`of ${limit}`). JSX text likewise, on its own line's spaces only: indentation and newlines are layout.
    const words = "/\\p{L}{2,}\\s+\\p{L}/u";
    const besideValue = "/\\p{L}{2,}\\s|\\s\\p{L}{2,}/u";
    const jsxWords = "/\\p{L}{2,}[^\\S\\n]+(\\S|$)|^[^\\S\\n]+\\p{L}{2,}|\\p{L}{2,}\\s+\\p{L}/u";
    const notText = "JSXAttribute[name.name=/^(className|class|key|data-.*)$/]";
    const skip = `:not(${notText} *, ExpressionStatement[directive] > Literal, ImportDeclaration > Literal, ExportAllDeclaration > Literal, ExportNamedDeclaration > Literal)`;
    const sentence = [
      `JSXText[value=${jsxWords}]`,
      `Literal[value=${words}]${skip}`,
      `TemplateElement[value.cooked=${besideValue}]${skip}`,
    ].map((selector) => ({
      selector,
      ...message("Write the sentence in components/results/copy.ts (copy-guide section 12) or components/actions/copy.ts (section 14), in openspec/changes/plain-language-copy/design.md; a renderer only calls it."),
    }));
    const tests = ["src/**/__tests__/**", "src/**/__fixtures__/**", "src/**/*.test.{ts,tsx}"];
    const fieldHome = ["src/lib/fields.ts", "src/components/kinds/storage/**"];
    const wordHome = ["src/components/kinds/storage/**", "src/app/storage/**", "src/lib/nav.ts"];
    const kindDir = "src/components/kinds/storage/**/*.{ts,tsx}";
    // Flat config: for a file, the LAST matching block that sets the rule wins whole. So each
    // block below lists every selector set its files owe, and the blocks partition the files.
    return [
      {
        // Everywhere: all three guards, minus where each is allowed.
        files: ["src/**/*.{ts,tsx}"],
        ignores: ["src/lib/queries/invalidation.ts", "src/components/results/*.tsx", "src/components/actions/*.tsx", ...fieldHome, ...wordHome, ...tests],
        rules: { "no-restricted-syntax": ["error", ...invalidation, ...roles, ...kindWord] },
      },
      {
        // The results and actions renderers: the everywhere set, and no inline sentence.
        files: ["src/components/results/*.tsx", "src/components/actions/*.tsx"],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...invalidation, ...roles, ...kindWord, ...sentence] },
      },
      {
        // invalidation.ts owns the invalidation map: fields + words only.
        files: ["src/lib/queries/invalidation.ts"],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...roles, ...kindWord] },
      },
      {
        // The storage kind: may say fields and words, still may not invalidate by hand.
        files: [kindDir],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...invalidation] },
      },
      {
        // app/storage and nav.ts: words allowed; fields and invalidation still refused.
        files: ["src/app/storage/**/*.{ts,tsx}", "src/lib/nav.ts"],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...invalidation, ...roles] },
      },
      {
        // fields.ts: may say fields; words and invalidation refused.
        files: ["src/lib/fields.ts"],
        ignores: tests,
        rules: { "no-restricted-syntax": ["error", ...invalidation, ...kindWord] },
      },
    ];
  })(),
  ...(() => {
    /** Fixtures are for tests: production code never imports one (a hand-made schema in a screen is a second source of the fact). */
    const fixtures = {
      group: ["**/__fixtures__", "**/__fixtures__/**"],
      message: "Fixtures are for tests only; production code reads the real source.",
    };
    const tests = ["src/**/__tests__/**", "src/**/__fixtures__/**", "src/**/*.test.{ts,tsx}"];
    return [
      {
        files: ["src/components/names/**/*.{ts,tsx}"],
        ignores: tests,
        rules: { "no-restricted-imports": ["error", { patterns: [fixtures] }] },
      },
      {
        /**
         * One path for a missing name (one-path W2d). "Unknown project" is said by
         * the names components only: projectLabel(name) where a string is needed,
         * <ProjectName> where a component is. Importing the phrase elsewhere is how
         * a second fallback starts. Its own rule, not no-restricted-syntax: a later
         * block setting that rule would replace the invalidation guard above.
         */
        files: ["src/**/*.{ts,tsx}"],
        ignores: ["src/components/names/**", ...tests],
        rules: {
          "no-restricted-imports": [
            "error",
            {
              // By pattern, so the alias and any relative path are both caught.
              patterns: [
                fixtures,
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
  })(),
];

export default eslintConfig;
