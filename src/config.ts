import type { Config } from "./types.js";

export function loadConfig(): Config {
  return {
    scanPath: process.env.SCAN_PATH || "/source",
    backupPath: process.env.BACKUP_PATH || "/backups",
    schedule: process.env.SCHEDULE || "0 3 * * *",
    retentionDays: parseInt(process.env.RETENTION_DAYS || "7", 10),
    retentionWeeks: parseInt(process.env.RETENTION_WEEKS || "4", 10),
    skipDirs: (process.env.SKIP_DIRS || "backup_service,backup_service_dev")
      .split(",")
      .map((d) => d.trim()),
  };
}
