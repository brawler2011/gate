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
            "@typescript-eslint/no-unused-vars": "warn",
            "@typescript-eslint/no-explicit-any": "warn",
            "react-hooks/exhaustive-deps": "warn",
            "react-hooks/set-state-in-effect": "off",
        },
    },
];

export default eslintConfig;

