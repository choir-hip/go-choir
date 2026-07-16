import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { lstat, readlink } from 'node:fs/promises';
import { dirname, isAbsolute, resolve, sep } from 'node:path';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const MAX_GIT_OUTPUT_BYTES = 16 * 1024 * 1024;

async function runGit(cwd, argumentsList) {
  const { stdout } = await execFileAsync('git', ['-C', cwd, ...argumentsList], {
    encoding: 'utf8',
    maxBuffer: MAX_GIT_OUTPUT_BYTES,
    windowsHide: true,
  });
  return stdout;
}

function fieldAfterSpaces(record, spaces) {
  let offset = -1;
  for (let index = 0; index < spaces; index += 1) {
    offset = record.indexOf(' ', offset + 1);
    if (offset === -1) return '';
  }
  return record.slice(offset + 1);
}

function changeState(kind, code) {
  if (kind === '?') return 'untracked';
  if (kind === 'u' || code.includes('U') || code === 'AA' || code === 'DD') return 'conflicted';
  const staged = code[0] !== '.';
  const unstaged = code[1] !== '.';
  if (staged && unstaged) return 'staged and unstaged';
  if (staged) return 'staged';
  return 'unstaged';
}

function parseStatus(output) {
  const fields = {};
  const untrackedPaths = [];
  const changedFiles = [];
  const records = output.split('\0');

  for (let index = 0; index < records.length; index += 1) {
    const record = records[index];
    if (!record) continue;
    if (record.startsWith('# ')) {
      const separator = record.indexOf(' ', 2);
      if (separator !== -1) fields[record.slice(2, separator)] = record.slice(separator + 1);
      continue;
    }
    if (record.startsWith('2 ')) {
      const code = record.slice(2, 4);
      changedFiles.push({ path: fieldAfterSpaces(record, 9), state: changeState('2', code), code });
      index += 1;
      continue;
    }
    if (record.startsWith('1 ')) {
      const code = record.slice(2, 4);
      changedFiles.push({ path: fieldAfterSpaces(record, 8), state: changeState('1', code), code });
      continue;
    }
    if (record.startsWith('u ')) {
      const code = record.slice(2, 4);
      changedFiles.push({ path: fieldAfterSpaces(record, 10), state: changeState('u', code), code });
      continue;
    }
    if (record.startsWith('? ')) {
      const path = record.slice(2);
      changedFiles.push({ path, state: 'untracked', code: '??' });
      untrackedPaths.push(path);
    }
  }

  changedFiles.sort((left, right) => left.path.localeCompare(right.path));

  const relation = /^\+(\d+) -(\d+)$/u.exec(fields['branch.ab'] ?? '');
  return {
    oid: fields['branch.oid'] ?? null,
    branch: fields['branch.head'] && fields['branch.head'] !== '(detached)'
      ? fields['branch.head']
      : null,
    detached: fields['branch.head'] === '(detached)',
    upstream: fields['branch.upstream'] ?? null,
    ahead: relation ? Number(relation[1]) : null,
    behind: relation ? Number(relation[2]) : null,
    dirtyFiles: changedFiles.length,
    changedFiles,
    untrackedPaths,
  };
}

function parseNumstat(output) {
  let addedLines = 0;
  let deletedLines = 0;
  let binaryFiles = 0;
  const files = new Map();
  const records = output.split('\0');

  for (let index = 0; index < records.length; index += 1) {
    const match = /^(-|\d+)\t(-|\d+)\t([\s\S]*)$/u.exec(records[index]);
    if (!match) continue;
    let path = match[3];
    if (path === '') {
      path = records[index + 2] ?? '';
      index += 2;
    }
    if (!path) continue;
    const binary = match[1] === '-' || match[2] === '-';
    if (binary) {
      binaryFiles += 1;
      files.set(path, { addedLines: null, deletedLines: null, binary: true });
      continue;
    }
    const added = Number(match[1]);
    const deleted = Number(match[2]);
    const previous = files.get(path);
    addedLines += added;
    deletedLines += deleted;
    files.set(path, {
      addedLines: (previous?.addedLines ?? 0) + added,
      deletedLines: (previous?.deletedLines ?? 0) + deleted,
      binary: false,
    });
  }

  return { addedLines, deletedLines, binaryFiles, files };
}

function resolveRepositoryPath(root, repositoryPath) {
  const absolute = resolve(root, repositoryPath);
  const rootPrefix = root.endsWith(sep) ? root : `${root}${sep}`;
  if (absolute !== root && !absolute.startsWith(rootPrefix)) {
    throw new Error('Git reported a path outside the worktree');
  }
  return absolute;
}

async function countUntrackedLines(root, repositoryPath) {
  const absolute = resolveRepositoryPath(root, repositoryPath);
  const fileStats = await lstat(absolute);
  if (fileStats.isSymbolicLink()) {
    const target = await readlink(absolute);
    return { addedLines: target.length > 0 ? 1 : 0, binary: false };
  }
  if (!fileStats.isFile()) return { addedLines: 0, binary: true };

  let bytes = 0;
  let newlines = 0;
  let lastByte = null;
  let binary = false;
  for await (const chunk of createReadStream(absolute)) {
    bytes += chunk.byteLength;
    for (const byte of chunk) {
      if (byte === 0) binary = true;
      if (byte === 10) newlines += 1;
      lastByte = byte;
    }
  }
  if (binary) return { addedLines: 0, binary: true };
  return { addedLines: bytes === 0 ? 0 : newlines + (lastByte === 10 ? 0 : 1), binary: false };
}

function withFingerprint(metadata) {
  return {
    ...metadata,
    fingerprint: createHash('sha256').update(JSON.stringify(metadata)).digest('hex'),
  };
}

export async function collectRepositoryMetadata(sourcePath) {
  const cwd = dirname(resolve(sourcePath));
  try {
    const [statusOutput, pathsOutput] = await Promise.all([
      runGit(cwd, ['status', '--porcelain=v2', '--branch', '-z', '--untracked-files=all']),
      runGit(cwd, ['rev-parse', '--path-format=absolute', '--show-toplevel', '--git-dir', '--git-common-dir']),
    ]);
    const status = parseStatus(statusOutput);
    const [worktreePath, gitDirectory, commonGitDirectory] = pathsOutput.trimEnd().split('\n');
    if (![worktreePath, gitDirectory, commonGitDirectory].every((value) => value && isAbsolute(value))) {
      throw new Error('Git did not report absolute repository paths');
    }

    const trackedNumstat = status.oid === '(initial)'
      ? `${await runGit(cwd, ['diff', '--numstat', '-z', '--cached', '--'])}${await runGit(cwd, ['diff', '--numstat', '-z', '--'])}`
      : await runGit(cwd, ['diff', '--numstat', '-z', 'HEAD', '--']);
    const tracked = parseNumstat(trackedNumstat);
    let addedLines = tracked.addedLines;
    let binaryFiles = tracked.binaryFiles;
    let unreadableFiles = 0;
    for (const repositoryPath of status.untrackedPaths) {
      try {
        const untracked = await countUntrackedLines(worktreePath, repositoryPath);
        addedLines += untracked.addedLines;
        if (untracked.binary) binaryFiles += 1;
        tracked.files.set(repositoryPath, {
          addedLines: untracked.binary ? null : untracked.addedLines,
          deletedLines: untracked.binary ? null : 0,
          binary: untracked.binary,
        });
      } catch {
        unreadableFiles += 1;
        tracked.files.set(repositoryPath, { addedLines: null, deletedLines: null, binary: false });
      }
    }

    let upstreamHead = null;
    if (status.upstream) {
      try {
        upstreamHead = (await runGit(cwd, ['rev-parse', '--short=12', status.upstream])).trim() || null;
      } catch {}
    }

    const changedFiles = await Promise.all(status.changedFiles.map(async (file) => {
      const lineDelta = tracked.files.get(file.path);
      let base;
      if (lineDelta) base = { ...file, ...lineDelta };
      else if (file.state === 'conflicted') base = { ...file, addedLines: null, deletedLines: null, binary: false };
      else base = { ...file, addedLines: 0, deletedLines: 0, binary: false };
      let lastModifiedAt = null;
      try {
        const fileStats = await lstat(resolveRepositoryPath(worktreePath, file.path));
        lastModifiedAt = fileStats.mtime.toISOString();
      } catch {}
      return { ...base, lastModifiedAt };
    }));

    return withFingerprint({
      available: true,
      branch: status.branch,
      detached: status.detached,
      head: status.oid && status.oid !== '(initial)' ? status.oid.slice(0, 12) : null,
      worktreePath,
      worktreeKind: resolve(gitDirectory) === resolve(commonGitDirectory) ? 'primary' : 'linked',
      upstream: status.upstream,
      upstreamHead,
      ahead: status.ahead,
      behind: status.behind,
      dirtyFiles: status.dirtyFiles,
      changedFiles,
      addedLines: unreadableFiles === 0 ? addedLines : null,
      deletedLines: unreadableFiles === 0 ? tracked.deletedLines : null,
      binaryFiles,
      unreadableFiles,
    });
  } catch {
    return withFingerprint({
      available: false,
      reason: 'Not a Git worktree or Git metadata is unreadable.',
    });
  }
}
