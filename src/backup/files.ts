import { execSync } from "node:child_process";
import { existsSync, statSync, readdirSync } from "node:fs";
import { join, relative, dirname, basename } from "node:path";
import { createWriteStream } from "node:fs";
import archiver from "archiver";
import type { DiscoveredService, BackupResult, BindMount } from "../types.js";

const SKIP_DIRS = new Set([
  "node_modules",
  ".git",
  "vendor",
  "__pycache__",
  ".cache",
  ".next",
  "dist",
  "build",
  "coverage",
  ".nyc_output",
]);

function getGitTrackedFiles(projectDir: string): Set<string> {
  try {
    const output = execSync(`git -C "${projectDir}" ls-files`, {
      encoding: "utf-8",
      timeout: 10000,
    });
    return new Set(output.split("\n").filter(Boolean));
  } catch {
    return new Set();
  }
}

function shouldIncludeFile(
  filePath: string,
  projectDir: string,
  trackedFiles: Set<string>,
  composeFile: string
): boolean {
  const rel = relative(projectDir, filePath);
  if (!rel) return false;

  const parts = rel.split("/");

  for (const part of parts) {
    if (SKIP_DIRS.has(part)) return false;
  }

  if (trackedFiles.has(rel) && rel !== composeFile && !rel.endsWith(".env")) {
    return false;
  }

  return true;
}

function collectFiles(
  dir: string,
  projectDir: string,
  trackedFiles: Set<string>,
  composeFile: string
): string[] {
  const files: string[] = [];

  if (!existsSync(dir)) return files;

  const entries = readdirSync(dir);

  for (const entry of entries) {
    const fullPath = join(dir, entry);
    if (!statSync(fullPath).isFile()) continue;

    if (shouldIncludeFile(fullPath, projectDir, trackedFiles, composeFile)) {
      files.push(fullPath);
    }
  }

  return files;
}

function collectDirs(
  dir: string,
  projectDir: string,
  trackedFiles: Set<string>,
  composeFile: string
): { path: string; files: string[] }[] {
  const result: { path: string; files: string[] }[] = [];

  if (!existsSync(dir)) return result;

  const entries = readdirSync(dir);

  for (const entry of entries) {
    if (SKIP_DIRS.has(entry)) continue;

    const fullPath = join(dir, entry);
    if (!statSync(fullPath).isDirectory()) continue;

    const files = collectFilesRecursive(
      fullPath,
      projectDir,
      trackedFiles,
      composeFile
    );
    if (files.length > 0) {
      result.push({ path: fullPath, files });
    }
  }

  return result;
}

function collectFilesRecursive(
  dir: string,
  projectDir: string,
  trackedFiles: Set<string>,
  composeFile: string
): string[] {
  const files: string[] = [];

  if (!existsSync(dir)) return files;

  const entries = readdirSync(dir);

  for (const entry of entries) {
    const fullPath = join(dir, entry);

    if (statSync(fullPath).isDirectory()) {
      if (SKIP_DIRS.has(entry)) continue;
      files.push(
        ...collectFilesRecursive(
          fullPath,
          projectDir,
          trackedFiles,
          composeFile
        )
      );
    } else {
      if (shouldIncludeFile(fullPath, projectDir, trackedFiles, composeFile)) {
        files.push(fullPath);
      }
    }
  }

  return files;
}

export async function backupFiles(
  service: DiscoveredService,
  backupPath: string
): Promise<BackupResult> {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  const archivePath = join(backupPath, `${service.name}/files_${timestamp}.tar.gz`);

  if (service.bindMounts.length === 0) {
    return {
      serviceName: service.name,
      type: "files",
      filePath: archivePath,
      sizeBytes: 0,
      timestamp: new Date(),
      status: "skipped",
      message: "No bind mounts found",
    };
  }

  const projectDir = service.projectDir;
  const isGitRepo = existsSync(join(projectDir, ".git"));
  const trackedFiles = isGitRepo
    ? getGitTrackedFiles(projectDir)
    : new Set<string>();

  const allFiles = new Map<string, string>();
  const composeFilePath = join(projectDir, service.composeFile);

  if (
    existsSync(composeFilePath) &&
    shouldIncludeFile(
      composeFilePath,
      projectDir,
      trackedFiles,
      service.composeFile
    )
  ) {
    allFiles.set(service.composeFile, composeFilePath);
  }

  for (const mount of service.bindMounts) {
    if (!mount.source.startsWith(projectDir)) continue;
    if (!existsSync(mount.source)) continue;

    const rel = relative(projectDir, mount.source);

    if (statSync(mount.source).isFile()) {
      if (
        shouldIncludeFile(
          mount.source,
          projectDir,
          trackedFiles,
          service.composeFile
        )
      ) {
        allFiles.set(rel, mount.source);
      }
    } else {
      const files = collectFilesRecursive(
        mount.source,
        projectDir,
        trackedFiles,
        service.composeFile
      );
      for (const file of files) {
        const fileRel = relative(projectDir, file);
        if (!allFiles.has(fileRel)) {
          allFiles.set(fileRel, file);
        }
      }
    }
  }

  if (allFiles.size === 0) {
    return {
      serviceName: service.name,
      type: "files",
      filePath: archivePath,
      sizeBytes: 0,
      timestamp: new Date(),
      status: "skipped",
      message: "No files to backup (all tracked by git)",
    };
  }

  const { mkdirSync } = await import("node:fs");
  mkdirSync(dirname(archivePath), { recursive: true });

  const sizeBytes = await new Promise<number>((resolve, reject) => {
    const output = createWriteStream(archivePath);
    const archive = archiver("tar", { gzip: true });

    output.on("close", () => resolve(archive.pointer()));
    archive.on("error", reject);

    archive.pipe(output);

    for (const [relPath, absPath] of allFiles) {
      archive.file(absPath, { name: relPath });
    }

    archive.finalize();
  });

  return {
    serviceName: service.name,
    type: "files",
    filePath: archivePath,
    sizeBytes,
    timestamp: new Date(),
    status: "success",
    message: `${allFiles.size} files backed up`,
  };
}
