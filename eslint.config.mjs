import tseslint from "typescript-eslint";
import react from "eslint-plugin-react";
import hooks from "eslint-plugin-react-hooks";
import imports from "eslint-plugin-import";

const eslintConfig = [
  {ignores: ["**/node_modules/**", "dist/**", "frontend/contracts/**", "**/.next/**"]},
  ...tseslint.configs.recommended,
  {
    files: ["**/*.ts", "**/*.tsx"],
    plugins: {react, "react-hooks": hooks, import: imports},
    settings: {react: {version: "detect"}, "import/internal-regex": "^@/"},
    rules: {...hooks.configs.recommended.rules},
  },
  {
    ignores: [
      ".next/**",
      "node_modules/**",
      "dist/**",
      "next-env.d.ts",
      "contracts/**",
    ],
  },
  {
    files: ["**/*.ts", "**/*.tsx"],
    rules: {
      // Тернарники, переменные и сравнения
      "curly": ["error", "all"],
      "brace-style": ["error", "1tbs", { "allowSingleLine": false }],
      "no-nested-ternary": "error",
      "semi": ["error", "always"],
      "eqeqeq": ["error", "always"],
      "prefer-const": "error",
      "no-var": "error",
      "no-console": ["warn", { allow: ["warn", "error"] }],
      "max-len": [
        "error",
        {
          code: 120,
          tabWidth: 2,
          ignoreUrls: true,
          ignoreStrings: true,
          ignoreTemplateLiterals: true,
          ignoreRegExpLiterals: true,
          ignoreComments: true,
        },
      ],

      // Форматирование и отступы (2 пробела)
      "indent": ["error", 2, { SwitchCase: 1 }],
      "no-mixed-spaces-and-tabs": "error",
      "no-multiple-empty-lines": ["error", { max: 1, maxEOF: 0, maxBOF: 0 }],

      // Сортировка и отступы импортов
      "import/order": [
        "error",
        {
          groups: ["builtin", "external", "internal", ["parent", "sibling"], "index", "type"],
          "newlines-between": "always",
          alphabetize: { order: "asc", caseInsensitive: true },
        },
      ],
      "import/newline-after-import": ["error", { count: 1 }],

      // Пробелы вокруг фигурных скобок {}
      "object-curly-spacing": ["error", "never"],

      // Стрелочные функции вместо function (разрешены генераторы function*)
      "func-style": ["error", "expression", { allowArrowFunctions: true }],
      "prefer-arrow-callback": "error",

      // Компоненты React & JSX
      "react/function-component-definition": [
        "error",
        {
          namedComponents: "arrow-function",
          unnamedComponents: "arrow-function",
        },
      ],
      "react/self-closing-comp": "error",
      "react/jsx-boolean-value": ["error", "never"],
      "react/jsx-curly-brace-presence": ["error", { props: "never", children: "never" }],

      // TypeScript & Hooks
      "@typescript-eslint/consistent-type-imports": ["error", { prefer: "type-imports" }],
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/explicit-module-boundary-types": [
        "error",
        {
          allowTypedFunctionExpressions: true,
          allowHigherOrderFunctions: true,
          allowDirectConstAssertionInArrowFunctions: true,
        },
      ],
      "react-hooks/exhaustive-deps": "warn",
      "react-hooks/set-state-in-effect": "off",

      // Prefer explicitly typed arrow components
      "no-restricted-syntax": [
        "error",
        {
          selector:
            "TSTypeReference[typeName.name='FC'], TSTypeReference[typeName.left.name='React'][typeName.right.name='FC'], TSTypeReference[typeName.name='FunctionComponent'], TSTypeReference[typeName.left.name='React'][typeName.right.name='FunctionComponent']",
          message:
            "Do not use React.FC or FC. Type props directly and specify an explicit return type (e.g. : JSX.Element or : Promise<JSX.Element>).",
        },
        {
          selector: "FunctionDeclaration[generator=false]",
          message: "Use arrow functions instead of function declarations.",
        },
        {
          selector: "FunctionExpression[generator=false]:not(MethodDefinition > FunctionExpression)",
          message: "Use arrow functions instead of function expressions.",
        },

      ],
    },
  },
];

export default eslintConfig;

