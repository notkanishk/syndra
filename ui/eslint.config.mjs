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
];

export default eslintConfig;
