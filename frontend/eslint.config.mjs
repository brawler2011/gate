import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import nextTypeScript from "eslint-config-next/typescript";

const eslintConfig = [
  ...nextCoreWebVitals,
  ...nextTypeScript,
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
      "react-hooks/exhaustive-deps": "warn",
      "react-hooks/set-state-in-effect": "off",

      // Запрет использования function (за исключением генераторов function*) и принудительная типизация Next.js
      "no-restricted-syntax": [
        "error",
        {
          selector: "FunctionDeclaration[generator=false]",
          message: "Use arrow functions instead of function declarations.",
        },
        {
          selector: "FunctionExpression[generator=false]:not(MethodDefinition > FunctionExpression)",
          message: "Use arrow functions instead of function expressions.",
        },
        {
          selector:
            "ExportNamedDeclaration > VariableDeclaration > VariableDeclarator[id.name=/^(revalidate|dynamic|dynamicParams|fetchCache|runtime|preferredRegion|maxDuration|metadata|viewport)$/][id.typeAnnotation=undefined]",
          message:
            "Next.js page/route constant must have an explicit type annotation (e.g. export const revalidate: number = 60;).",
        },
        {
          selector:
            "ExportNamedDeclaration > VariableDeclaration > VariableDeclarator[id.name=/^(generateMetadata|generateStaticParams|generateViewport|generateSitemaps)$/][id.typeAnnotation=undefined][init.returnType=undefined]",
          message:
            "Next.js special function must have an explicit type annotation or return type.",
        },
      ],
    },
  },
];

export default eslintConfig;

