import { $ } from "bun";
import { parseArgs } from "util";
import { existsSync, rmSync, statSync } from "fs";
import { createGzip } from "zlib";
import { createReadStream, createWriteStream } from "fs";
import { pipeline } from "stream/promises";
import { join } from "path";

// Configuration
const DEFAULT_SERVER = "217.12.38.213";
const DEFAULT_USER = "root";
const DEFAULT_IMAGE_NAME = "gate149-frontend";
const DEFAULT_ARCHIVE_NAME = "gate149-frontend.tar.gz";

// Parse arguments
const { values } = parseArgs({
  args: Bun.argv.slice(2),
  options: {
    Server: { type: "string", default: DEFAULT_SERVER },
    User: { type: "string", default: DEFAULT_USER },
    ImageName: { type: "string", default: DEFAULT_IMAGE_NAME },
    ArchiveName: { type: "string", default: DEFAULT_ARCHIVE_NAME },
    AutoDeploy: { type: "boolean", default: false },
    KeepArchive: { type: "boolean", default: false },
    StartFrom: { type: "string", default: "build" },
  },
  strict: false, // Allow other flags/args to pass through if needed
});

const config = {
  Server: values.Server as string,
  User: values.User as string,
  ImageName: values.ImageName as string,
  ArchiveName: values.ArchiveName as string,
  AutoDeploy: values.AutoDeploy as boolean,
  KeepArchive: values.KeepArchive as boolean,
  StartFrom: values.StartFrom as string,
};

// State
let currentStep = "";
let scriptSuccess = false;
const tarPath = join(import.meta.dir, "gate149-frontend.tar");
const gzPath = join(import.meta.dir, config.ArchiveName);

// Helpers
async function run(command: string[], options: { cwd?: string } = {}) {
  const proc = Bun.spawn(command, {
    stdio: ["inherit", "inherit", "inherit"],
    env: { ...process.env, FORCE_COLOR: "1" },
    cwd: options.cwd,
  });

  const exitCode = await proc.exited;
  if (exitCode !== 0) {
    throw new Error(`Command '${command[0]}' failed with exit code ${exitCode}`);
  }
}

function cleanupTempFiles(force = false) {
  if (config.KeepArchive && !force) {
    console.log("\x1b[33mKeeping archive files (-KeepArchive flag is set)\x1b[0m");
    return;
  }

  let cleaned = false;
  if (existsSync(tarPath)) {
    try {
      rmSync(tarPath, { force: true });
      console.log(`\x1b[90m  Removed: ${tarPath}\x1b[0m`);
      cleaned = true;
    } catch (e) {}
  }
  if (existsSync(gzPath)) {
    try {
      rmSync(gzPath, { force: true });
      console.log(`\x1b[90m  Removed: ${gzPath}\x1b[0m`);
      cleaned = true;
    } catch (e) {}
  }

  if (cleaned) {
    console.log("\x1b[33mCleanup complete.\x1b[0m");
  }
}

function showErrorHelp(step: string, errorMessage: string) {
  console.log("");
  console.log("\x1b[31m========================================\x1b[0m");
  console.log("\x1b[31mERROR: Deployment failed!\x1b[0m");
  console.log("\x1b[31m========================================\x1b[0m");
  console.log("");
  console.log(`\x1b[31mFailed at: ${step}\x1b[0m`);
  console.log(`\x1b[31mError: ${errorMessage}\x1b[0m`);
  console.log("");

  switch (step) {
    case "build":
      console.log("\x1b[33mWhat happened:\x1b[0m");
      console.log("  Docker image build failed");
      console.log("");
      console.log("\x1b[33mPossible causes:\x1b[0m");
      console.log("  - Dockerfile syntax error");
      console.log("  - Missing dependencies");
      console.log("  - .env.production not found or invalid");
      console.log("  - Docker daemon not running");
      break;
    case "save":
      console.log("\x1b[33mWhat happened:\x1b[0m");
      console.log("  Could not save Docker image to tar file");
      break;
    case "compress":
      console.log("\x1b[33mWhat happened:\x1b[0m");
      console.log("  Could not compress tar file to gzip");
      break;
    case "upload":
      console.log("\x1b[33mWhat happened:\x1b[0m");
      console.log("  Could not upload archive to server");
      console.log("");
      console.log("\x1b[36mHow to fix:\x1b[0m");
      console.log(`  1. Test SSH: ssh ${config.User}@${config.Server}`);
      break;
    case "deploy":
      console.log("\x1b[33mWhat happened:\x1b[0m");
      console.log("  Server deployment script failed");
      console.log("");
      console.log("\x1b[36mHow to fix:\x1b[0m");
      console.log(`  1. SSH to server: ssh ${config.User}@${config.Server}`);
      console.log("  2. Check logs: docker compose logs frontend");
      break;
  }
}

// Step mapping
const steps: Record<string, number> = {
  build: 1,
  save: 2,
  compress: 3,
  upload: 4,
  deploy: 5,
};

const startStep = steps[config.StartFrom] || 1;

async function main() {
  try {
    // Step 1: Build
    if (startStep <= 1) {
      currentStep = "build";
      console.log("\x1b[36m========================================\x1b[0m");
      console.log("\x1b[36mStep 1: Building Docker image...\x1b[0m");
      console.log("\x1b[36m========================================\x1b[0m");

      // Load .env.production (находится на уровень выше)
      const envFilePath = join(import.meta.dir, "../.env.production");
      const envFile = Bun.file(envFilePath);
      if (await envFile.exists()) {
        console.log(`\x1b[32mLoading environment variables from ${envFilePath}...\x1b[0m`);
        const content = await envFile.text();
        for (const line of content.split("\n")) {
          const match = line.match(/^([^=]+)=(.*)$/);
          if (match) {
            const key = match[1].trim();
            const value = match[2].trim();
            process.env[key] = value;
            console.log(`\x1b[90m  ${key} = ${value}\x1b[0m`);
          }
        }
      } else {
        console.log("\x1b[33mWarning: .env.production not found!\x1b[0m");
      }

      console.log("\x1b[36mBuilding Docker image...\x1b[0m");
      
      const composeFile = join(import.meta.dir, "docker-compose.yml");
      
      // Use Bun.spawn for TTY inheritance
      await run(["docker", "compose", "-f", composeFile, "--env-file", envFilePath, "build"]);
      console.log("");
    }

    // Step 2: Save
    if (startStep <= 2) {
      currentStep = "save";
      console.log("\x1b[36m========================================\x1b[0m");
      console.log("\x1b[36mStep 2: Saving Docker image to tar...\x1b[0m");
      console.log("\x1b[36m========================================\x1b[0m");

      const composedImageName = "deploy-frontend"; // From docker-compose (папка deploy + service name)
      // Use Bun.spawn for consistency (though save output is minimal)
      await run(["docker", "save", "-o", tarPath, composedImageName]);
      
      console.log(`\x1b[32mSaved to: ${tarPath}\x1b[0m`);
      console.log("");
    }

    // Step 3: Compress
    if (startStep <= 3) {
      currentStep = "compress";
      console.log("\x1b[36m========================================\x1b[0m");
      console.log("\x1b[36mStep 3: Compressing with gzip...\x1b[0m");
      console.log("\x1b[36m========================================\x1b[0m");

      if (!existsSync(tarPath)) {
        throw new Error(`Tar file not found: ${tarPath}`);
      }

      console.log("\x1b[33mCompressing...\x1b[0m");
      const source = createReadStream(tarPath);
      const destination = createWriteStream(gzPath);
      const gzip = createGzip();

      await pipeline(source, gzip, destination);

      // Remove uncompressed tar
      rmSync(tarPath, { force: true });

      const stats = statSync(gzPath);
      const fileSizeMB = (stats.size / (1024 * 1024)).toFixed(2);
      console.log(`\x1b[32mArchive created: ${config.ArchiveName} (${fileSizeMB} MB)\x1b[0m`);
      console.log("");
    }

    // Step 4: Upload
    if (startStep <= 4) {
      currentStep = "upload";
      console.log("\x1b[36m========================================\x1b[0m");
      console.log("\x1b[36mStep 4: Uploading to server...\x1b[0m");
      console.log("\x1b[36m========================================\x1b[0m");

      if (!existsSync(gzPath)) {
        throw new Error(`Archive not found: ${gzPath}`);
      }

      const remotePath = `${config.User}@${config.Server}:/tmp/${config.ArchiveName}`;
      console.log(`\x1b[90mUploading image to ${remotePath}\x1b[0m`);
      
      // scp usually shows a progress bar, so use run() with inherit
      await run(["scp", gzPath, remotePath]);

      const deployScript = join(import.meta.dir, "server-deploy.sh");
      if (existsSync(deployScript)) {
        console.log("\x1b[90mUploading deployment script...\x1b[0m");
        await run(["scp", deployScript, `${config.User}@${config.Server}:/tmp/server-deploy.sh`]);
        console.log("\x1b[32mDeployment script uploaded\x1b[0m");
      }

      console.log("");
    }

    // Step 5: Deploy
    if (startStep <= 5) {
      console.log("\x1b[32m========================================\x1b[0m");
      console.log("\x1b[32mUpload complete!\x1b[0m");
      console.log("\x1b[32m========================================\x1b[0m");
      console.log("");

      if (config.AutoDeploy) {
        currentStep = "deploy";
        console.log("\x1b[36m========================================\x1b[0m");
        console.log("\x1b[36mStep 5: Running deployment on server...\x1b[0m");
        console.log("\x1b[36m========================================\x1b[0m");
        console.log("");

        try {
          await run([
            "ssh",
            "-o", "ConnectTimeout=10",
            `${config.User}@${config.Server}`,
            "bash /tmp/server-deploy.sh"
          ]);
        } catch (e) {
          console.log("");
          console.log("\x1b[31mServer deployment failed. See error above.\x1b[0m");
          console.log("\x1b[33mTo retry: bun run deploy.ts --StartFrom deploy --AutoDeploy\x1b[0m");
          process.exit(1);
        }

        console.log("");
        console.log("\x1b[32m========================================\x1b[0m");
        console.log("\x1b[32mFull deployment complete!\x1b[0m");
        console.log("\x1b[32m========================================\x1b[0m");
      } else {
        console.log("\x1b[33mTo deploy on the server run:\x1b[0m");
        console.log("");
        console.log(`  ssh ${config.User}@${config.Server}`);
        console.log("  bash /tmp/server-deploy.sh");
        console.log("");
        console.log("\x1b[33mOr in one command:\x1b[0m");
        console.log(`\x1b[36m  ssh ${config.User}@${config.Server} bash /tmp/server-deploy.sh\x1b[0m`);
        console.log("");
        console.log("\x1b[33mOr run with --AutoDeploy:\x1b[0m");
        console.log("\x1b[36m  bun run deploy.ts --AutoDeploy\x1b[0m");
        console.log("");
      }
    }

    scriptSuccess = true;

    console.log("\x1b[90mAvailable options:\x1b[0m");
    console.log("\x1b[90m  bun run deploy.ts --StartFrom build       # Start from beginning\x1b[0m");
    console.log("\x1b[90m  bun run deploy.ts --StartFrom upload      # Skip to upload\x1b[0m");
    console.log("\x1b[90m  bun run deploy.ts --KeepArchive           # Keep .tar.gz file after upload\x1b[0m");
    console.log("\x1b[90m  bun run deploy.ts --AutoDeploy            # Auto-deploy after upload\x1b[0m");
    console.log("");

  } catch (error: any) {
    showErrorHelp(currentStep, error.message || error);
    process.exitCode = 1;
  } finally {
    // Cleanup
    console.log("");
    const hasFiles = existsSync(tarPath) || existsSync(gzPath);

    if (scriptSuccess) {
      if (hasFiles) {
        if (config.KeepArchive) {
          console.log(`\x1b[33mArchive kept at: ${gzPath}\x1b[0m`);
        } else {
          console.log("\x1b[90mCleaning up...\x1b[0m");
          cleanupTempFiles();
        }
      }
    } else {
      if (hasFiles) {
         console.log("\x1b[33mTemporary files found:\x1b[0m");
         if (existsSync(tarPath)) console.log(`\x1b[90m  - ${tarPath}\x1b[0m`);
         if (existsSync(gzPath)) console.log(`\x1b[90m  - ${gzPath}\x1b[0m`);
         console.log("");
         console.log("To clean up manually, delete these files.");
      }
    }
  }
}

main();
