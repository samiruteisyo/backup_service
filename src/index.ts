import { loadConfig } from "./config.js";
import { discoverServices } from "./discover.js";
import { parseComposeFile } from "./parser.js";
import { backupFiles } from "./backup/files.js";
import { backupDatabase } from "./backup/database.js";
import { rotateAllBackups } from "./storage.js";
import type { DiscoveredService, BackupResult } from "./types.js";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function logResult(result: BackupResult) {
  const icon =
    result.status === "success"
      ? "[OK]"
      : result.status === "skipped"
        ? "[SKIP]"
        : "[FAIL]";
  const size =
    result.status === "success" ? ` (${formatBytes(result.sizeBytes)})` : "";
  console.log(`  ${icon} ${result.type}: ${result.message}${size}`);
}

async function runBackup(dryRun = false): Promise<void> {
  const config = loadConfig();
  const composeFiles = discoverServices(config);

  if (composeFiles.length === 0) {
    console.log("No services discovered.");
    return;
  }

  console.log(`Discovered ${composeFiles.length} service(s):\n`);

  const services: DiscoveredService[] = [];
  const results: BackupResult[] = [];

  for (const composePath of composeFiles) {
    const service = parseComposeFile(composePath);
    services.push(service);

    console.log(`[${service.name}]`);
    console.log(`  Compose: ${service.composeFile}`);
    console.log(`  Database: ${service.database ? `${service.database.type} (${service.database.containerName})` : "none"}`);
    console.log(
      `  Bind mounts: ${service.bindMounts.length > 0 ? service.bindMounts.map((m) => m.source).join(", ") : "none"}`
    );

    if (dryRun) {
      console.log("  (dry-run: skipping backup)\n");
      continue;
    }

    const dbResult = await backupDatabase(service, config.backupPath);
    results.push(dbResult);
    logResult(dbResult);

    const fileResult = await backupFiles(service, config.backupPath);
    results.push(fileResult);
    logResult(fileResult);

    console.log("");
  }

  if (!dryRun) {
    console.log("Rotating old backups...\n");
    const rotation = rotateAllBackups(
      config,
      services.map((s) => s.name)
    );
    for (const r of rotation) {
      if (r.deleted > 0) {
        console.log(`  [${r.service}] deleted ${r.deleted}, kept ${r.kept}`);
      }
    }

    const successCount = results.filter((r) => r.status === "success").length;
    const skippedCount = results.filter((r) => r.status === "skipped").length;
    const errorCount = results.filter((r) => r.status === "error").length;
    const totalSize = results.reduce((sum, r) => sum + r.sizeBytes, 0);

    console.log(`\nBackup complete: ${successCount} succeeded, ${skippedCount} skipped, ${errorCount} failed — total ${formatBytes(totalSize)}`);
  }
}

async function main() {
  const args = process.argv.slice(2);
  const isManual = args.includes("--manual");
  const isDryRun = args.includes("--dry-run");

  if (isManual || isDryRun) {
    await runBackup(isDryRun);
    process.exit(0);
  }

  const config = loadConfig();
  const { default: cron } = await import("node-cron");

  console.log(`Backup service started. Schedule: ${config.schedule}`);

  await runBackup();

  cron.schedule(config.schedule, () => {
    console.log(`\n--- Scheduled backup at ${new Date().toISOString()} ---`);
    runBackup().catch((err) => {
      console.error("Scheduled backup failed:", err);
    });
  });
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
