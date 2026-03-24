import { readdirSync, statSync, unlinkSync } from "node:fs";
import { join } from "node:path";
import type { Config } from "./types.js";

interface BackupFile {
  name: string;
  path: string;
  mtime: Date;
  sizeBytes: number;
}

function getAgeInDays(date: Date): number {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  return diffMs / (1000 * 60 * 60 * 24);
}

function getStartOfWeek(date: Date): Date {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - d.getDay());
  return d;
}

export function rotateBackups(
  config: Config,
  serviceName: string
): { kept: number; deleted: number } {
  const serviceBackupDir = join(config.backupPath, serviceName);

  let kept = 0;
  let deleted = 0;

  try {
    const files = readdirSync(serviceBackupDir);
    const backups: BackupFile[] = [];

    for (const file of files) {
      const filePath = join(serviceBackupDir, file);
      const stat = statSync(filePath);
      if (!stat.isFile()) continue;

      backups.push({
        name: file,
        path: filePath,
        mtime: stat.mtime,
        sizeBytes: stat.size,
      });
    }

    const weeklyKept = new Set<string>();

    for (const backup of backups) {
      const ageDays = getAgeInDays(backup.mtime);

      if (ageDays <= config.retentionDays) {
        kept++;
        continue;
      }

      if (ageDays <= config.retentionWeeks * 7) {
        const weekStart = getStartOfWeek(backup.mtime).toISOString();
        const weekKey = `${backup.name.split("_")[0]}_${weekStart}`;

        if (!weeklyKept.has(weekKey)) {
          weeklyKept.add(weekKey);
          kept++;
          continue;
        }
      }

      try {
        unlinkSync(backup.path);
        deleted++;
      } catch {
        kept++;
      }
    }
  } catch {
    // Directory doesn't exist yet, nothing to rotate
  }

  return { kept, deleted };
}

export function rotateAllBackups(
  config: Config,
  serviceNames: string[]
): { service: string; kept: number; deleted: number }[] {
  return serviceNames.map((name) => ({
    service: name,
    ...rotateBackups(config, name),
  }));
}
