const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

// Helper to deep merge objects
function deepMerge(target, source) {
  for (const key of Object.keys(source)) {
    if (source[key] instanceof Object && key in target) {
      Object.assign(source[key], deepMerge(target[key], source[key]));
    }
  }
  Object.assign(target || {}, source);
  return target;
}

function mergeSpecs(mainPath, authPath, outputPath) {
  if (!fs.existsSync(mainPath)) {
    throw new Error(`Main specification not found at: ${mainPath}`);
  }
  if (!fs.existsSync(authPath)) {
    throw new Error(`Auth specification not found at: ${authPath}`);
  }

  const mainSpec = yaml.load(fs.readFileSync(mainPath, 'utf8'));
  const authSpec = yaml.load(fs.readFileSync(authPath, 'utf8'));

  // Merge paths
  if (authSpec.paths) {
    mainSpec.paths = mainSpec.paths || {};
    deepMerge(mainSpec.paths, authSpec.paths);
  }

  // Merge components
  if (authSpec.components) {
    mainSpec.components = mainSpec.components || {};
    if (authSpec.components.schemas) {
      mainSpec.components.schemas = mainSpec.components.schemas || {};
      deepMerge(mainSpec.components.schemas, authSpec.components.schemas);
    }
    if (authSpec.components.securitySchemes) {
      mainSpec.components.securitySchemes = mainSpec.components.securitySchemes || {};
      deepMerge(mainSpec.components.securitySchemes, authSpec.components.securitySchemes);
    }
  }

  // Write merged specification
  fs.writeFileSync(outputPath, yaml.dump(mainSpec, { lineWidth: -1 }));
}

try {
  // 1. Merge core/v1/core.yaml + core/v1/auth.yaml -> core/v1/openapi.yaml
  const coreMain = path.join(__dirname, '../core/v1/core.yaml');
  const authPath = path.join(__dirname, '../core/v1/auth.yaml');
  const coreOutput = path.join(__dirname, '../core/v1/openapi.yaml');

  console.log('Merging core contracts...');
  mergeSpecs(coreMain, authPath, coreOutput);

  // 2. Merge gateway/v1/gateway.yaml + core/v1/auth.yaml -> gateway/v1/openapi.yaml
  const gatewayMain = path.join(__dirname, '../gateway/v1/gateway.yaml');
  const gatewayOutput = path.join(__dirname, '../gateway/v1/openapi.yaml');

  console.log('Merging gateway contracts...');
  mergeSpecs(gatewayMain, authPath, gatewayOutput);

  console.log('Merge complete!');
} catch (error) {
  console.error('Error merging OpenAPI specifications:', error.message);
  process.exit(1);
}
