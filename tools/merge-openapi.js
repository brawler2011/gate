const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

/**
 * Merge OpenAPI specifications from blogs and core into gateway
 */
function mergeOpenAPI() {
  console.log('Reading source OpenAPI specifications...');
  
  // Read blogs and core OpenAPI files
  const blogsPath = path.join(__dirname, '../blogs/v1/openapi.yaml');
  const corePath = path.join(__dirname, '../core/v1/openapi.yaml');
  
  if (!fs.existsSync(blogsPath)) {
    throw new Error(`Blogs OpenAPI not found at: ${blogsPath}`);
  }
  
  if (!fs.existsSync(corePath)) {
    throw new Error(`Core OpenAPI not found at: ${corePath}`);
  }
  
  const blogsSpec = yaml.load(fs.readFileSync(blogsPath, 'utf8'));
  const coreSpec = yaml.load(fs.readFileSync(corePath, 'utf8'));
  
  console.log(`  - Blogs: ${Object.keys(blogsSpec.paths || {}).length} paths`);
  console.log(`  - Core: ${Object.keys(coreSpec.paths || {}).length} paths`);
  
  // Create gateway specification
  const gatewaySpec = {
    openapi: '3.0.3',
    info: {
      title: 'Gateway API',
      description: 'Unified API Gateway combining Blogs and Core services',
      version: '1.0.0'
    },
    paths: {},
    components: {
      securitySchemes: {},
      schemas: {}
    }
  };
  
  // Service prefixes for routing
  const SERVICE_PREFIXES = {
    'blogs': '/blogs',
    'core': '/tester'
  };
  
  // Merge paths with prefixes
  console.log('Merging paths with prefixes...');
  
  // Add blogs paths with /blogs prefix
  const blogsPaths = {};
  for (const [path, methods] of Object.entries(blogsSpec.paths || {})) {
    const newPath = SERVICE_PREFIXES.blogs + path;
    blogsPaths[newPath] = methods;
  }
  console.log(`  - Blogs: ${Object.keys(blogsPaths).length} paths with /blogs prefix`);
  
  // Add core paths with /tester prefix
  const corePaths = {};
  for (const [path, methods] of Object.entries(coreSpec.paths || {})) {
    const newPath = SERVICE_PREFIXES.core + path;
    corePaths[newPath] = methods;
  }
  console.log(`  - Core: ${Object.keys(corePaths).length} paths with /tester prefix`);
  
  // Merge into gateway
  Object.assign(gatewaySpec.paths, blogsPaths);
  Object.assign(gatewaySpec.paths, corePaths);
  
  // Merge security schemes
  console.log('Merging security schemes...');
  if (blogsSpec.components?.securitySchemes) {
    Object.assign(gatewaySpec.components.securitySchemes, blogsSpec.components.securitySchemes);
  }
  if (coreSpec.components?.securitySchemes) {
    Object.assign(gatewaySpec.components.securitySchemes, coreSpec.components.securitySchemes);
  }
  
  // Merge schemas (models)
  console.log('Merging schemas...');
  if (blogsSpec.components?.schemas) {
    Object.assign(gatewaySpec.components.schemas, blogsSpec.components.schemas);
  }
  if (coreSpec.components?.schemas) {
    // Core schemas override blogs schemas if there are duplicates
    Object.assign(gatewaySpec.components.schemas, coreSpec.components.schemas);
  }
  
  // Merge responses if they exist
  if (blogsSpec.components?.responses || coreSpec.components?.responses) {
    gatewaySpec.components.responses = {};
    if (blogsSpec.components?.responses) {
      Object.assign(gatewaySpec.components.responses, blogsSpec.components.responses);
    }
    if (coreSpec.components?.responses) {
      Object.assign(gatewaySpec.components.responses, coreSpec.components.responses);
    }
  }
  
  // Merge parameters if they exist
  if (blogsSpec.components?.parameters || coreSpec.components?.parameters) {
    gatewaySpec.components.parameters = {};
    if (blogsSpec.components?.parameters) {
      Object.assign(gatewaySpec.components.parameters, blogsSpec.components.parameters);
    }
    if (coreSpec.components?.parameters) {
      Object.assign(gatewaySpec.components.parameters, coreSpec.components.parameters);
    }
  }
  
  // Create output directory
  const outputDir = path.join(__dirname, '../gateway/v1');
  if (!fs.existsSync(outputDir)) {
    console.log('Creating gateway/v1 directory...');
    fs.mkdirSync(outputDir, { recursive: true });
  }
  
  // Write merged specification
  const outputPath = path.join(outputDir, 'openapi.yaml');
  console.log(`Writing merged specification to ${outputPath}...`);
  fs.writeFileSync(outputPath, yaml.dump(gatewaySpec, { lineWidth: -1 }));
  
  console.log('✓ Gateway OpenAPI specification created successfully');
  console.log(`  - Total paths: ${Object.keys(gatewaySpec.paths).length}`);
  console.log(`  - Total schemas: ${Object.keys(gatewaySpec.components.schemas).length}`);
  console.log(`  - Total security schemes: ${Object.keys(gatewaySpec.components.securitySchemes).length}`);
}

// Run merge
try {
  mergeOpenAPI();
} catch (error) {
  console.error('Error merging OpenAPI specifications:', error.message);
  process.exit(1);
}
