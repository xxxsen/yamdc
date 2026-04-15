import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import tseslint from "typescript-eslint";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  globalIgnores([".next/**", "out/**", "build/**", "coverage/**", "next-env.d.ts"]),

  // ── type-checked rules (equivalent to Go errcheck / staticcheck) ──
  {
    files: ["src/**/*.ts", "src/**/*.tsx"],
    extends: [tseslint.configs.strictTypeCheckedOnly],
    languageOptions: {
      parserOptions: { projectService: true },
    },
    rules: {
      // -- tuning the strict preset --

      "@typescript-eslint/no-unnecessary-condition": "error",

      // allow `void promise` for fire-and-forget patterns used in effects
      "@typescript-eslint/no-confusing-void-expression": ["error", {
        ignoreArrowShorthand: true,
        ignoreVoidOperator: true,
      }],
      // relax for JSX event handler attributes (onClick={async () => ...})
      "@typescript-eslint/no-misused-promises": ["error", {
        checksVoidReturn: { attributes: false },
      }],
      // allow ${number} and ${boolean} in template literals
      "@typescript-eslint/restrict-template-expressions": ["error", {
        allowNumber: true,
        allowBoolean: true,
      }],
      // the project intentionally uses `as` casts on API JSON responses
      "@typescript-eslint/no-unsafe-argument": "off",
      "@typescript-eslint/no-unsafe-assignment": "off",
      "@typescript-eslint/no-unsafe-call": "off",
      "@typescript-eslint/no-unsafe-member-access": "off",
      "@typescript-eslint/no-unsafe-return": "off",
      "@typescript-eslint/no-non-null-assertion": "error",
      "@typescript-eslint/unified-signatures": "off",
      "@typescript-eslint/no-dynamic-delete": "off",
      "@typescript-eslint/no-empty-object-type": "off",
      "@typescript-eslint/no-invalid-void-type": "off",
    },
  },

  // ── strict rules for source files ──
  {
    files: ["src/**/*.ts", "src/**/*.tsx"],
    rules: {
      // correctness — equivalent to Go errcheck / govet
      "@typescript-eslint/no-shadow": "error",
      "@typescript-eslint/no-unused-vars": ["error", {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
        caughtErrorsIgnorePattern: "^_",
      }],
      "@typescript-eslint/consistent-type-imports": ["error", {
        prefer: "type-imports",
        fixStyle: "separate-type-imports",
      }],

      // maintainability — equivalent to Go gocyclo / nestif
      "complexity": ["error", 50],
      "max-depth": ["error", 5],

      // engineering discipline — equivalent to Go forbidigo
      "no-console": ["error", { allow: ["warn", "error", "debug"] }],

      // project serves images via its own API; next/image optimization is not applicable
      "@next/next/no-img-element": "off",
    },
  },

  // ── relax for test files (mirrors Go _test.go exclusions) ──
  {
    files: ["src/**/__tests__/**", "src/**/*.test.ts", "src/**/*.test.tsx"],
    rules: {
      "@typescript-eslint/no-floating-promises": "off",
      "@typescript-eslint/no-unnecessary-condition": "off",
      "@typescript-eslint/no-unnecessary-type-parameters": "off",
      "@typescript-eslint/require-await": "off",
      "@typescript-eslint/no-shadow": "off",
      "@typescript-eslint/consistent-type-imports": "off",
      "complexity": "off",
      "max-depth": "off",
      "no-console": "off",
    },
  },
]);

export default eslintConfig;
