import { readdirSync, statSync } from "node:fs";
import { join, basename } from "node:path";
import type { Config } from "./types.js";

const COMPOSE_FILES = [
  "docker-compose.yml",
  "docker-compose.yaml",
  "compose.yml",
  "compose.yaml",
];

export function discoverServices(config: Config): string[] {
  const entries = readdirSync(config.scanPath);
  const services: string[] = [];

  for (const entry of entries) {
    const fullPath = join(config.scanPath, entry);
    if (!statSync(fullPath).isDirectory()) continue;
    if (config.skipDirs.includes(entry)) continue;

    for (const composeFile of COMPOSE_FILES) {
      const composePath = join(fullPath, composeFile);
      try {
        statSync(composePath);
        services.push(composePath);
        break;
      } catch {}
    }
  }

  return services;
}
