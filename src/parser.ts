import { readFileSync, existsSync, statSync } from "node:fs";
import { dirname, basename, join, resolve } from "node:path";
import { parse } from "yaml";
import type { DiscoveredService, DatabaseInfo, BindMount } from "./types.js";

interface ComposeService {
  image?: string;
  container_name?: string;
  environment?: Record<string, string> | string[];
  volumes?: (string | { source: string; target: string; type: string })[];
}

const DB_IMAGE_PATTERNS: Record<
  string,
  { type: DatabaseInfo["type"]; defaultPort: number }
> = {
  postgres: { type: "postgres", defaultPort: 5432 },
  mysql: { type: "mysql", defaultPort: 3306 },
  mariadb: { type: "mariadb", defaultPort: 3306 },
  mongo: { type: "mongo", defaultPort: 27017 },
  mongodb: { type: "mongo", defaultPort: 27017 },
};

const DB_ENV_MAP: Record<
  DatabaseInfo["type"],
  { database: string[]; user: string[]; password: string[] }
> = {
  postgres: {
    database: ["POSTGRES_DB", "POSTGRES_DATABASE"],
    user: ["POSTGRES_USER"],
    password: ["POSTGRES_PASSWORD"],
  },
  mysql: {
    database: ["MYSQL_DATABASE"],
    user: ["MYSQL_USER", "MARIADB_USER"],
    password: ["MYSQL_PASSWORD", "MARIADB_PASSWORD"],
  },
  mariadb: {
    database: ["MYSQL_DATABASE", "MARIADB_DATABASE"],
    user: ["MYSQL_USER", "MARIADB_USER"],
    password: ["MYSQL_PASSWORD", "MARIADB_PASSWORD"],
  },
  mongo: {
    database: ["MONGO_INITDB_DATABASE"],
    user: ["MONGO_INITDB_ROOT_USERNAME"],
    password: ["MONGO_INITDB_ROOT_PASSWORD"],
  },
};

function parseEnvFile(envPath: string): Record<string, string> {
  if (!existsSync(envPath)) return {};
  const content = readFileSync(envPath, "utf-8");
  const env: Record<string, string> = {};
  for (const line of content.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eqIdx = trimmed.indexOf("=");
    if (eqIdx === -1) continue;
    const key = trimmed.slice(0, eqIdx).trim();
    let value = trimmed.slice(eqIdx + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    env[key] = value;
  }
  return env;
}

function resolveEnvValue(
  value: string,
  env: Record<string, string>
): string | undefined {
  const match = value.match(/^\$\{(.+?)(?::-([^}]*))?\}$/);
  if (!match) return value;
  const key = match[1];
  const fallback = match[2];
  if (env[key] !== undefined) return env[key];
  return fallback || undefined;
}

function detectDatabase(
  serviceName: string,
  service: ComposeService,
  projectEnv: Record<string, string>
): DatabaseInfo | null {
  const image = service.image || "";
  const imageLower = image.toLowerCase();

  for (const [pattern, info] of Object.entries(DB_IMAGE_PATTERNS)) {
    if (imageLower.includes(pattern)) {
      const envObj: Record<string, string> = {};
      if (Array.isArray(service.environment)) {
        for (const e of service.environment) {
          const eqIdx = e.indexOf("=");
          if (eqIdx !== -1) {
            envObj[e.slice(0, eqIdx).trim()] = e.slice(eqIdx + 1).trim();
          }
        }
      } else if (service.environment) {
        for (const [k, v] of Object.entries(service.environment)) {
          envObj[k] = resolveEnvValue(v, projectEnv) || v;
        }
      }

      const envMap = DB_ENV_MAP[info.type];
      return {
        type: info.type,
        containerName: service.container_name || serviceName,
        serviceName,
        credentials: {
          database: envObj[envMap.database[0]] || envObj[envMap.database[1]],
          user: envObj[envMap.user[0]] || envObj[envMap.user[1]],
          password: envObj[envMap.password[0]] || envObj[envMap.password[1]],
          port: info.defaultPort,
        },
      };
    }
  }
  return null;
}

function extractBindMounts(
  serviceName: string,
  service: ComposeService,
  composeDir: string
): BindMount[] {
  const mounts: BindMount[] = [];
  if (!service.volumes) return mounts;

  for (const vol of service.volumes) {
    let source: string;
    let target: string;

    if (typeof vol === "string") {
      const parts = vol.split(":");
      if (parts.length < 2) continue;
      source = parts[0];
      target = parts[1];
    } else {
      if (vol.type !== "bind") continue;
      source = vol.source;
      target = vol.target;
    }

    if (source.startsWith(".") || (source.length > 0 && source[0] !== "/")) {
      source = resolve(composeDir, source);
    }

    if (!source.startsWith(composeDir)) continue;
    if (!existsSync(source)) continue;

    mounts.push({ source, containerPath: target });
  }

  return mounts;
}

export function parseComposeFile(composePath: string): DiscoveredService {
  const content = readFileSync(composePath, "utf-8");
  const compose = parse(content) as { services?: Record<string, ComposeService> };

  const projectDir = dirname(composePath);
  const serviceName = basename(projectDir);
  const projectEnv = parseEnvFile(join(projectDir, ".env"));

  let database: DatabaseInfo | null = null;
  const bindMountMap = new Map<string, BindMount>();

  for (const [name, service] of Object.entries(compose.services || {})) {
    const db = detectDatabase(name, service, projectEnv);
    if (db) {
      database = db;
    }
    for (const mount of extractBindMounts(name, service, projectDir)) {
      bindMountMap.set(mount.source, mount);
    }
  }

  const bindMounts = [...bindMountMap.values()];

  return {
    name: serviceName,
    composePath,
    projectDir,
    database,
    bindMounts,
    composeFile: basename(composePath),
  };
}
