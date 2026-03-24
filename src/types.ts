export interface DatabaseInfo {
  type: "postgres" | "mysql" | "mariadb" | "mongo";
  containerName: string;
  serviceName: string;
  credentials: {
    host?: string;
    port?: number;
    database?: string;
    user?: string;
    password?: string;
  };
}

export interface BindMount {
  source: string;
  containerPath: string;
}

export interface DiscoveredService {
  name: string;
  composePath: string;
  projectDir: string;
  database: DatabaseInfo | null;
  bindMounts: BindMount[];
  composeFile: string;
}

export interface BackupResult {
  serviceName: string;
  type: "database" | "files";
  filePath: string;
  sizeBytes: number;
  timestamp: Date;
  status: "success" | "skipped" | "error";
  message?: string;
}

export interface Config {
  scanPath: string;
  backupPath: string;
  schedule: string;
  retentionDays: number;
  retentionWeeks: number;
  skipDirs: string[];
}
