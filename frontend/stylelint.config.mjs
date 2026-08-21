/** @type {import('stylelint').Config} */
const config = {
  extends: ["stylelint-config-standard"],
  ignoreFiles: [
    ".next/**",
    "node_modules/**",
    "dist/**",
    "contracts/**",
  ],
  rules: {
    "selector-class-pattern": null,
    "keyframes-name-pattern": null,
    "property-no-vendor-prefix": null,
    "selector-pseudo-class-no-unknown": [
      true,
      {
        ignorePseudoClasses: ["global", "local"],
      },
    ],
    "custom-property-pattern": null,
    "media-feature-range-notation": null,
    "media-query-no-invalid": null,
    "at-rule-no-unknown": [
      true,
      {
        ignoreAtRules: ["mixin", "define-mixin"],
      },
    ],
  },
};

export default config;
