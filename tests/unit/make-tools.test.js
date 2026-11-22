import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, delimiter } from 'node:path';

test('quiet command reports progress and retains complete failure output', () => {
  const success = spawnSync('bash', ['scripts/quiet-run.sh', 'Example', 'bash', '-c', 'echo hidden']);
  assert.equal(success.status, 0);
  assert.match(success.stdout.toString(), /Example: passed/);
  assert.doesNotMatch(success.stdout.toString(), /hidden/);

  const failure = spawnSync('bash', ['scripts/quiet-run.sh', 'Example', 'bash', '-c', 'echo first; echo last >&2; exit 7']);
  assert.equal(failure.status, 7);
  assert.match(failure.stderr.toString(), /Example: failed \(exit 7\)/);
  assert.match(failure.stderr.toString(), /first[\s\S]*last/);
});

test('Make watch targets forward arguments and preserve failures', () => {
  const dir = mkdtempSync(join(tmpdir(), 'make-watch-'));
  try {
    const gh = join(dir, 'gh');
    writeFileSync(gh, '#!/usr/bin/env bash\nprintf "<%s>\\n" "$@"\nexit "${FAKE_GH_STATUS:-0}"\n');
    chmodSync(gh, 0o755);
    const env = { ...process.env, PATH: `${dir}${delimiter}${process.env.PATH}`, FAKE_GH_STATUS: '0' };
    const invoke = (...args) => spawnSync('make', ['-s', 'PR=', 'RUN=', ...args], { encoding: 'utf8', env });
    assert.match(invoke('pr-checks').stdout, /<pr>\n<checks>\n<--watch>/);
    assert.match(invoke('pr-checks', 'PR=9').stdout, /<checks>\n<9>\n<--watch>/);
    assert.match(invoke('run-watch', 'RUN=123').stdout, /<watch>\n<123>\n<--compact>\n<--exit-status>/);
    assert.notEqual(invoke('run-watch').status, 0);
    env.FAKE_GH_STATUS = '1';
    assert.notEqual(invoke('pr-checks', 'PR=9').status, 0);
    assert.notEqual(invoke('run-watch', 'RUN=123').status, 0);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
