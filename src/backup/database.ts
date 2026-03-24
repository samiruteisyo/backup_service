import { execSync } from "node:child_process";
import { mkdirSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import type { DiscoveredService, BackupResult, DatabaseInfo } from "../types.js";

function resolveContainerName(db: DatabaseInfo, projectName: string): string | null {
  const candidates = [
    db.containerName,
    `${projectName}-${db.serviceName}-1`,
    `${projectName}_${db.serviceName}_1`,
    `${projectName}-${db.serviceName}`,
    `${projectName}_${db.serviceName}`,
  ];

  for (const name of candidates) {
    try {
      const output = execSync(
        `docker inspect -f '{{.State.Running}}' ${name}`,
        { encoding: "utf-8", timeout: 10000, stdio: ["pipe", "pipe", "pipe"] }
      );
      if (output.trim() === "true") return name;
    } catch {}
  }

  return null;
}

function getDumpCommand(db: DatabaseInfo): string {
  const { credentials, containerName, type } = db;
  const dbName = credentials.database || "";
  const user = credentials.user || "";
  const pass = credentials.password || "";

  switch (type) {
    case "postgres":
      return `docker exec ${containerName} pg_dump -U ${user} ${dbName}`;
    case "mysql":
      return `docker exec ${containerName} mysqldump -u ${user} -p'${pass}' ${dbName}`;
    case "mariadb":
      return `docker exec ${containerName} mariadb-dump -u ${user} -p'${pass}' ${dbName}`;
    case "mongo":
      return `docker exec ${containerName} mongodump --db ${dbName} --username ${user} --password '${pass}' --archive`;
  }
}

export async function backupDatabase(
  service: DiscoveredService,
  backupPath: string
): Promise<BackupResult> {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  const extension = service.database?.type === "mongo" ? "archive.gz" : "sql.gz";
  const archivePath = join(
    backupPath,
    `${service.name}/db_${timestamp}.${extension}`
  );

  if (!service.database) {
    return {
      serviceName: service.name,
      type: "database",
      filePath: archivePath,
      sizeBytes: 0,
      timestamp: new Date(),
      status: "skipped",
      message: "No database detected",
    };
  }

  const db = service.database;
  const resolvedName = resolveContainerName(db, service.name);

  if (!resolvedName) {
    return {
      serviceName: service.name,
      type: "database",
      filePath: archivePath,
      sizeBytes: 0,
      timestamp: new Date(),
      status: "skipped",
      message: `Database container '${db.serviceName}' is not running (tried: ${db.containerName}, ${service.name}-${db.serviceName}-1)`,
    };
  }

  db.containerName = resolvedName;

  mkdirSync(dirname(archivePath), { recursive: true });

  try {
    const dumpCmd = getDumpCommand(db);
    execSync(`${dumpCmd} | gzip > ${archivePath}`, {
      timeout: 300000,
      encoding: "utf-8",
    });

    const { statSync } = await import("node:fs");
    const sizeBytes = statSync(archivePath).size;

    return {
      serviceName: service.name,
      type: "database",
      filePath: archivePath,
      sizeBytes,
      timestamp: new Date(),
      status: "success",
      message: `${db.type} database '${db.credentials.database}' backed up from '${db.containerName}'`,
    };
  } catch (err) {
    return {
      serviceName: service.name,
      type: "database",
      filePath: archivePath,
      sizeBytes: 0,
      timestamp: new Date(),
      status: "error",
      message: `Backup failed: ${err instanceof Error ? err.message : String(err)}`,
    };
  }
}
