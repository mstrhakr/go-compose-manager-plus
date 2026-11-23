import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test, expect } from '@playwright/test';

let fixture;
test.beforeAll(() => {
  const directory = mkdtempSync(join(tmpdir(), 'cmp-dashboard-preview-'));
  try {
    execFileSync('go', ['test', './internal/web', '-run', '^TestSyntheticDashboardPreview$', '-count=1'], {
      env: { ...process.env, CMP_UI_PREVIEW_DIR: directory },
      stdio: 'pipe',
    });
    fixture = readFileSync(join(directory, 'dashboard.html'), 'utf8');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('synthetic dashboard renders current metrics and interactive aggregates', async ({ page }, testInfo) => {
  await page.route('**/synthetic-dashboard-preview', route => route.fulfill({
    status: 200, contentType: 'text/html', body: fixture,
  }));
  await page.goto('/synthetic-dashboard-preview');
  await expect(page.getByText('Synthetic preview data only.', { exact: false })).toBeVisible();
  const charts = page.locator('[data-chart]');
  await expect(charts).toHaveCount(3);
  const cpu = charts.nth(0);
  const data = await cpu.evaluate(element => JSON.parse(element.dataset.chartSeries));
  expect(Math.abs(Date.now() - data.xMax)).toBeLessThan(60_000);
  expect(data.series).toHaveLength(1);
  expect(data.series[0].points).toHaveLength(120);
  expect(data.series[0].points.filter(point => point.y !== null).length).toBeGreaterThan(100);
  await expect(cpu.locator('polyline')).toHaveCount(2);
  await expect(charts.nth(2).locator('polyline')).toHaveCount(3);
  const svg = cpu.locator('svg');
  await svg.focus();
  await page.keyboard.press('ArrowLeft');
  await expect(cpu.locator('.chart-cursor')).toHaveAttribute('x1',
    await charts.nth(2).locator('.chart-cursor').getAttribute('x1'));
  await svg.hover({ position: { x: 100, y: 40 } });
  await expect(cpu.locator('.chart-tooltip')).toContainText('Collected container total');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

  if (testInfo.project.name === 'chromium') {
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width: 2560, height: 1440 });
      await page.emulateMedia({ colorScheme: theme });
      await page.screenshot({ path: `test-results/dashboard-synthetic-${theme}-2560x1440.png` });
    }
  }
});
