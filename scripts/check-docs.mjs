import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, readdirSync, rmSync, statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import assert from 'node:assert/strict';

const root = mkdtempSync(join(tmpdir(), 'cmp-docs-'));
const baseURL = 'https://example.invalid/compose-manager-plus/';

function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? files(path) : [path];
  });
}

try {
  for (const contracts of [false, true]) {
    for (const development of [false, true]) {
      const destination = join(root, `${contracts}-${development}`);
      const config = ['hugo.toml'];
      if (contracts) config.push('hugo.contracts.toml');
      if (development) config.push('hugo.development.toml');
      execFileSync('hugo', ['--source', 'docs', '--config', config.join(','), '--destination', destination,
        '--minify', '--panicOnWarning', '--baseURL', baseURL], { stdio: 'inherit' });
      const paths = files(destination);
      const has = path => paths.includes(join(destination, path));
      assert.ok(has('getting-started/index.html'));
      assert.ok(has('examples/compose.yaml'));
      const home = readFileSync(join(destination, 'index.html'), 'utf8');
      assert.match(home, /data-search-open/, 'theme search control is missing');
      assert.match(home, /theme-toggle/, 'theme mode control is missing');
      assert.match(home, /© 2026 Compose Manager Plus/, 'project footer branding is missing');
      assert.ok(paths.some(path => /\.search-data\..+\.json$/.test(path)), 'offline search index is missing');
      const gettingStarted = readFileSync(join(destination, 'getting-started', 'index.html'), 'utf8');
      assert.match(gettingStarted, /class="hextra-toc/, 'theme table of contents is missing');
      assert.match(gettingStarted, /aria-label="Table of contents"/, 'table of contents is not labeled');
      assert.match(gettingStarted, /hx:sticky/, 'table of contents is not sticky');
      assert.match(gettingStarted, /href=#run-locally/, 'heading navigation link is missing');
      assert.match(gettingStarted, /id=run-locally/, 'heading anchor is missing');
      const themeScript = paths
        .filter(path => /\/js\/main\.min\..+\.js$/.test(path))
        .map(path => readFileSync(path, 'utf8'))
        .join('\n');
      assert.match(themeScript, /IntersectionObserver/, 'scroll-aware table of contents behavior is missing');
      const themeStylesheet = paths.find(path => /\/css\/compiled\/main\.min\..+\.css$/.test(path));
      assert.ok(themeStylesheet, 'theme stylesheet is missing');
      assert.match(readFileSync(themeStylesheet, 'utf8'), /--primary-hue:210deg/, 'brand accent is missing');
      assert.equal(has('contracts/index.html'), contracts);
      for (const page of ['architecture', 'development', 'deployment']) {
        assert.equal(has(`${page}/index.html`), development, `${page} publication mismatch`);
      }
      for (const path of paths.filter(path => path.endsWith('.html'))) {
        const html = readFileSync(path, 'utf8');
        for (const match of html.matchAll(/(?:href|src)="([^"]+)"/g)) {
          const target = new URL(match[1], new URL(path.slice(destination.length + 1), baseURL));
          if (target.origin !== new URL(baseURL).origin || !target.pathname.startsWith(new URL(baseURL).pathname)) continue;
          const relative = decodeURIComponent(target.pathname.slice(new URL(baseURL).pathname.length));
          const resolved = join(destination, relative);
          let found = false;
          try {
            found = statSync(resolved).isDirectory() ? statSync(join(resolved, 'index.html')).isFile() : statSync(resolved).isFile();
          } catch (error) {
            if (error.code !== 'ENOENT') throw error;
          }
          assert.ok(found, `broken local link in ${path}: ${match[1]}`);
        }
      }
    }
  }
  const reused = join(root, 'true-true');
  execFileSync('hugo', ['--source', 'docs', '--config', 'hugo.toml', '--destination', reused,
    '--cleanDestinationDir', '--minify', '--panicOnWarning', '--baseURL', baseURL], { stdio: 'inherit' });
  const remaining = files(reused);
  for (const page of ['contracts', 'architecture', 'development', 'deployment']) {
    assert.ok(!remaining.includes(join(reused, page, 'index.html')), `stale ${page} page remains`);
  }
  console.log('Documentation: public and opt-in variants and local links checked');
} finally {
  rmSync(root, { recursive: true, force: true });
}
