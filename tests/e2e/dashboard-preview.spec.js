import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test, expect } from '@playwright/test';

let fixture;
let observabilityFixtures;
let settingsFixture;
test.beforeAll(() => {
  const directory = mkdtempSync(join(tmpdir(), 'cmp-dashboard-preview-'));
  try {
    execFileSync('go', ['test', './internal/web', '-run', '^TestSyntheticDashboardPreview$', '-count=1'], {
      env: { ...process.env, CMP_UI_PREVIEW_DIR: directory },
      stdio: 'pipe',
    });
    fixture = readFileSync(join(directory, 'dashboard.html'), 'utf8');
    settingsFixture = readFileSync(join(directory, 'settings.html'), 'utf8');
    observabilityFixtures = Array.from({ length: 6 }, (_, index) =>
      readFileSync(join(directory, `observability-${index}.html`), 'utf8'));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

function serveObservability(route) {
  const query = new URL(route.request().url()).searchParams;
  const selected = query.get('container');
  const index = query.get('q') === 'database' ? 4 : query.get('q') === 'missing' ? 5 :
    selected ? ['a', 'b', 'c', 'd'].findIndex(letter => selected === letter.repeat(64)) : 0;
  return route.fulfill({
    status: index < 0 ? 404 : 200, contentType: 'text/html',
    body: index < 0 ? 'Container history unavailable' : observabilityFixtures[index],
  });
}

test('compact observability keeps long container lists inside their scrolling pane', async ({ page }) => {
  await page.route('**/containers/observability*', serveObservability);
  await page.goto('/containers/observability');
  const pane = page.getByRole('complementary', { name: 'Filtered containers' });
  const names = pane.getByRole('navigation', { name: 'Filtered containers' });
  await names.evaluate(element => {
    const original = element.firstElementChild;
    for (let index = 0; index < 80; index++) {
      const link = original.cloneNode(true);
      link.removeAttribute('aria-current');
      link.lastElementChild.textContent = `extra-container-${index}`;
      element.append(link);
    }
  });
  for (const width of [900, 620, 390]) {
    await page.setViewportSize({ width, height: 800 });
    const before = await pane.evaluate(element => ({
      height: element.clientHeight, content: element.scrollHeight,
      bottom: element.getBoundingClientRect().bottom,
    }));
    expect(before.height).toBeLessThanOrEqual(240);
    expect(before.content).toBeGreaterThan(before.height);
    const toolbar = await page.locator('.workspace-toolbar').boundingBox();
    expect(toolbar.y).toBeGreaterThanOrEqual(before.bottom);
    await page.locator('.console-main').focus();
    await names.getByRole('link').last().focus();
    await expect.poll(() => pane.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
    const last = await names.getByRole('link').last().boundingBox();
    const bounds = await pane.boundingBox();
    expect(last.y + last.height).toBeLessThanOrEqual(bounds.y + bounds.height + 1);
    await pane.evaluate(element => { element.scrollTop = 0; });
  }
});

test('single-container observability has a vertical synchronized timeline and stacked records', async ({ page }, testInfo) => {
  await page.route('**/containers/observability*', serveObservability);
  await page.goto('/containers/observability');
  const timeline = page.locator('[data-timeline]');
  const containers = page.getByRole('navigation', { name: 'Filtered containers' });
  await expect(containers.getByRole('link')).toHaveCount(4);
  await expect(containers.getByRole('link').first()).toHaveAttribute('aria-current', 'page');
  await expect(timeline).toHaveAttribute('data-container', 'a'.repeat(64));
  const charts = page.locator('[data-chart]');
  await expect(charts).toHaveCount(3);
  const assertIsolation = async letter => {
    const data = await charts.evaluateAll(elements => elements.map(element => JSON.parse(element.dataset.chartSeries)));
    for (const [index, chart] of data.entries()) {
      if (index === 2) {
        expect(chart.series.map(series => series.label)).toEqual(['Receive', 'Transmit']);
      } else {
        expect(chart.series).toHaveLength(1);
        expect(chart.series[0].label).toBe(`Container ${letter.repeat(12)}`);
      }
    }
  };
  await assertIsolation('a');
  const filterPanel = page.getByRole('complementary', { name: 'Inventory filters' });
  const namePanel = page.getByRole('complementary', { name: 'Filtered containers' });
  await expect(filterPanel.getByLabel('Search')).toBeVisible();
  await expect(containers.getByRole('link', { name: 'api', exact: true })).toBeVisible();
  const slider = page.getByRole('slider', { name: 'Selected time in the last hour' });
  await expect(slider).toHaveAttribute('aria-orientation', 'vertical');
  const bounds = await slider.boundingBox();
  expect(bounds.height).toBeGreaterThan(bounds.width * 5);
  if (testInfo.project.name === 'mobile-chromium') {
    await slider.tap({ position: { x: bounds.width / 2, y: bounds.height / 2 } });
  } else {
    await slider.click({ position: { x: bounds.width / 2, y: bounds.height / 2 } });
  }
  const fraction = (Number(await slider.inputValue()) - Number(await timeline.getAttribute('data-start'))) /
    (Number(await timeline.getAttribute('data-end')) - Number(await timeline.getAttribute('data-start')));
  expect(fraction).toBeGreaterThan(0.4);
  expect(fraction).toBeLessThan(0.6);
  const clock = timeline.locator('[data-timeline-output]');
  const rail = timeline.locator('.timeline-rail');
  const clockBounds = await clock.boundingBox();
  const railBounds = await rail.boundingBox();
  expect(Math.abs(clockBounds.y + clockBounds.height / 2 - (railBounds.y + railBounds.height / 2))).toBeLessThan(8);
  await expect(clock).toHaveAttribute('title', await slider.getAttribute('aria-valuetext'));
  await expect(clock).toHaveText(/\d{2}:\d{2}:\d{2}/);
  const chartBounds = await charts.evaluateAll(elements => elements.map(element => {
    const bounds = element.getBoundingClientRect();
    return { x: bounds.x, y: bounds.y, bottom: bounds.bottom };
  }));
  for (let index = 1; index < chartBounds.length; index++) {
    expect(chartBounds[index].x).toBe(chartBounds[0].x);
    expect(chartBounds[index].y).toBeGreaterThan(chartBounds[index - 1].bottom);
  }
  const restart = page.locator('.timeline-event').filter({ hasText: 'Restarted' });
  if (testInfo.project.name === 'mobile-chromium') await restart.tap();
  else await restart.click();
  const selectedTime = await restart.getAttribute('data-timeline-select');
  await expect(timeline).toHaveAttribute('data-selected', selectedTime);
  for (const svg of await charts.locator('svg').all()) {
    await expect(svg).toHaveAttribute('aria-valuenow', selectedTime);
  }
  const marker = page.locator('[data-timeline-marker][data-kind="restart"]');
  const position = await marker.evaluate(element => ({ left: element.style.left, top: parseFloat(element.style.top) }));
  expect(position.left).toBe('');
  expect(position.top).toBeCloseTo(30, 1);
  await slider.focus();
  await page.keyboard.press('Home');
  await expect(slider).toHaveValue(await timeline.getAttribute('data-start'));
  let selectedBounds = await clock.boundingBox();
  let axisBounds = await rail.boundingBox();
  expect(selectedBounds.y + selectedBounds.height).toBeLessThanOrEqual(axisBounds.y + axisBounds.height);
  await page.keyboard.press('ArrowUp');
  expect(Number(await slider.inputValue())).toBe(Number(await timeline.getAttribute('data-start')) + 1000);
  await timeline.getByRole('button', { name: 'Select end of displayed hour' }).click();
  await expect(slider).toHaveValue(await timeline.getAttribute('data-end'));
  await expect(timeline).toHaveAttribute('data-selected', await timeline.getAttribute('data-end'));
  for (const svg of await charts.locator('svg').all()) {
    await expect(svg).toHaveAttribute('aria-valuenow', await timeline.getAttribute('data-end'));
  }
  selectedBounds = await clock.boundingBox();
  axisBounds = await rail.boundingBox();
  expect(selectedBounds.y).toBeGreaterThanOrEqual(axisBounds.y);
  const logsLabel = page.locator('.observability-record-options').getByText('Container logs', { exact: true });
  if (testInfo.project.name === 'mobile-chromium') await logsLabel.tap();
  else await logsLabel.click();
  await expect(page.locator('.observability-logs')).toBeVisible();
  await expect(page.locator('.observability-logs')).toContainText('Container logs are not collected');
  await expect(page.locator('.timeline-events')).toBeHidden();
  await containers.getByRole('link').nth(1).click();
  await expect(timeline).toHaveAttribute('data-container', 'b'.repeat(64));
  await assertIsolation('b');
  await expect(page.locator('.timeline-events')).toContainText('Out of memory');
  await expect(page.locator('.timeline-events')).not.toContainText('Restarted');
  await expect(page.getByRole('checkbox', { name: /Container/ })).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await filterPanel.getByLabel('Search').fill('database');
  await filterPanel.getByRole('button', { name: 'Apply filters' }).click();
  await expect(containers.getByRole('link')).toHaveCount(1);
  await expect(containers.getByRole('link', { name: 'database', exact: true })).toHaveAttribute('aria-current', 'page');
  await expect(timeline).toHaveAttribute('data-container', 'b'.repeat(64));
  await expect(containers.getByRole('link')).toHaveAttribute('href', /q=database/);
  await filterPanel.getByLabel('Search').fill('missing');
  await filterPanel.getByRole('button', { name: 'Apply filters' }).click();
  await expect(containers.getByRole('link')).toHaveCount(0);
  await expect(namePanel).toContainText('No containers match these filters');
  await expect(page.locator('[data-chart-series]')).toHaveCount(0);
  await page.goto('/containers/observability?container=' + 'b'.repeat(64));
  if (testInfo.project.name === 'chromium') {
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width: 2560, height: 1440 });
      await page.emulateMedia({ colorScheme: theme });
      await page.screenshot({ path: `test-results/observability-single-container-${theme}-2560x1440.png` });
      await page.setViewportSize({ width: 1256, height: 668 });
      await expect(charts.nth(2)).toBeInViewport();
      await expect(page.locator('.observability-records')).toBeInViewport();
      await expect(page.locator('.timeline-event').first()).toBeInViewport({ ratio: 0.9 });
      await expect(page.locator('.timeline-event').nth(1)).toBeInViewport({ ratio: 0.9 });
      const contrast = await clock.evaluate(element => {
        const luminance = color => {
          const channels = color.match(/\d+/g).slice(0, 3).map(value => {
            const channel = Number(value) / 255;
            return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
          });
          return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
        };
        const style = getComputedStyle(element);
        const text = luminance(style.color);
        const background = luminance(style.backgroundColor);
        return (Math.max(text, background) + 0.05) / (Math.min(text, background) + 0.05);
      });
      expect(contrast).toBeGreaterThanOrEqual(4.5);
      await page.screenshot({ path: `test-results/observability-single-container-${theme}-reference.png` });
      await page.locator('.timeline-event').filter({ hasText: 'Out of memory' }).click();
      await page.screenshot({ path: `test-results/observability-selected-event-${theme}-reference.png` });
      await timeline.getByRole('button', { name: 'Select end of displayed hour' }).click();
    }
    for (const width of [1440, 1024, 901, 900, 620, 390]) {
      await page.setViewportSize({ width, height: 900 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await expect(containers.getByRole('link').nth(1)).toBeVisible();
    }
    await page.setViewportSize({ width: 1440, height: 700 });
    const filterBounds = await filterPanel.boundingBox();
    const nameBounds = await namePanel.boundingBox();
    expect(nameBounds.x).toBeGreaterThanOrEqual(filterBounds.x + filterBounds.width);
    expect(nameBounds.width).toBeLessThan(filterBounds.width);
    expect((await slider.boundingBox()).x).toBeGreaterThanOrEqual(nameBounds.x + nameBounds.width);
    const sidebarTop = await containers.evaluate(element => element.getBoundingClientRect().top);
    const railTop = await page.locator('.timeline-panel').evaluate(element => element.getBoundingClientRect().top);
    await page.locator('.console-main').evaluate(element => { element.scrollTop = 180; });
    expect(await containers.evaluate(element => element.getBoundingClientRect().top)).toBe(sidebarTop);
    expect(await page.locator('.timeline-panel').evaluate(element => element.getBoundingClientRect().top)).toBeLessThanOrEqual(railTop);
    await expect(slider).toBeInViewport();
    await page.locator('.console-main').evaluate(element => { element.scrollTop = 0; });
    await page.setViewportSize({ width: 390, height: 844 });
    const contentBounds = await filterPanel.locator('.sidebar-content').boundingBox();
    expect(contentBounds.y + contentBounds.height).toBeLessThanOrEqual((await namePanel.boundingBox()).y);
    const networkSeries = charts.nth(2).locator('.chart-series-details');
    await networkSeries.locator('summary').click();
    await expect(networkSeries.getByRole('button', { name: 'Receive', exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await networkSeries.locator('summary').click();
    await charts.nth(2).locator('.chart-data-details summary').click();
    await expect(charts.nth(2).locator('.chart-table-scroll')).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await charts.nth(2).locator('.chart-data-details summary').click();
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({ path: 'test-results/observability-single-container-mobile.png', fullPage: true });
  }
});

test('stats settings show preset values and unlock only the current values under Custom', async ({ page, browser, baseURL }, testInfo) => {
  const serveSettings = route => route.fulfill({ status: 200, contentType: 'text/html', body: settingsFixture });
  await page.route('**/settings', serveSettings);
  await page.goto('/settings');
  const scope = page.getByLabel('Collection scope');
  const interval = page.getByLabel('Sampling interval (for example 5s)');
  await expect(page.getByRole('navigation', { name: 'Settings categories' })
    .getByRole('link', { name: 'Containers' })).toHaveAttribute('aria-current', 'page');
  await expect(interval).toHaveValue('1s');
  await expect(interval).toHaveAttribute('readonly', '');
  await expect(scope).toBeHidden();
  await page.getByRole('radio', { name: 'Low', exact: true }).press('Space');
  await expect(interval).toHaveValue('15s');
  await page.getByRole('radio', { name: 'Custom', exact: true }).press('Space');
  await expect(scope).toBeVisible();
  await expect(interval).toBeEditable();
  await expect(page.getByLabel('Collection scope')).toHaveValue('activated');
  await expect(page.getByLabel('Sampling interval (for example 5s)')).toHaveValue('15s');
  await expect(page.locator('[data-metrics-preset]:visible')).toHaveCount(0);
  await page.getByLabel('Collection scope').selectOption('all');
  await page.getByLabel('Sampling interval (for example 5s)').fill('3s');
  await page.getByRole('radio', { name: 'Medium', exact: true }).press('Space');
  await expect(interval).toHaveValue('5s');
  await page.getByRole('radio', { name: 'Custom', exact: true }).press('Space');
  await expect(page.getByLabel('Collection scope')).toHaveValue('activated');
  await expect(page.getByLabel('Sampling interval (for example 5s)')).toHaveValue('5s');
  await expect(page.getByLabel('Storage target')).toHaveValue('10.48576');
  await page.getByRole('radio', { name: 'High', exact: true }).press('Space');
  await page.getByLabel('Storage target').fill('20');
  await expect(page.getByRole('radio', { name: 'Custom', exact: true })).toBeChecked();
  await expect(page.getByLabel('Collection scope')).toHaveValue('all');
  await expect(page.getByLabel('Sampling interval (for example 5s)')).toHaveValue('1s');
  if (testInfo.project.name === 'chromium') {
    for (const theme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: theme });
      for (const width of [1256, 390]) {
        await page.setViewportSize({ width, height: 900 });
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
        await page.screenshot({ path: `test-results/settings-observability-${theme}-${width}.png`, fullPage: true });
      }
    }
  }
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false });
  try {
    const fallback = await context.newPage();
    await fallback.route('**/settings', serveSettings);
    await fallback.goto('/settings');
    await fallback.getByRole('radio', { name: 'Medium', exact: true }).press('Space');
    await expect(fallback.getByRole('textbox', { name: 'Sampling interval', exact: true })).toHaveValue('5s');
    await fallback.getByRole('radio', { name: 'Custom', exact: true }).press('Space');
    await expect(fallback.getByLabel('Collection scope')).toBeVisible();
    await expect(fallback.locator('noscript .metrics-help')).toBeVisible();
    await expect(fallback.locator('noscript .metrics-help')).toContainText('save a preset before selecting Custom');
  } finally {
    await context.close();
  }
});

test('single-container navigation and record selectors work without JavaScript', async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false });
  try {
    await context.route('**/containers/observability*', serveObservability);
    const page = await context.newPage();
    await page.goto('/containers/observability');
    await expect(page.locator('[data-timeline-latest]')).toBeHidden();
    await page.getByRole('navigation', { name: 'Filtered containers' }).getByRole('link', { name: 'database', exact: true }).click();
    await expect(page.locator('[data-timeline]')).toHaveAttribute('data-container', 'b'.repeat(64));
    await expect(page.locator('.timeline-event').filter({ hasText: 'Out of memory' })).toBeVisible();
    await page.locator('.observability-record-options').getByText('Container logs', { exact: true }).click();
    await expect(page.locator('.observability-logs')).toBeVisible();
    await expect(page.locator('.timeline-events')).toBeHidden();
    await page.getByLabel('Search', { exact: true }).fill('database');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page.getByRole('navigation', { name: 'Filtered containers' }).getByRole('link')).toHaveCount(1);
  } finally {
    await context.close();
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
  await page.getByRole('radio', { name: 'CPU', exact: true }).focus();
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('radio', { name: 'Memory', exact: true })).toBeChecked();
  await expect(charts.nth(1)).toBeVisible();
  await expect(cpu).toBeHidden();
  const networkOption = page.getByRole('radio', { name: 'Network', exact: true });
  if (testInfo.project.name === 'mobile-chromium') {
    await page.getByText('Network', { exact: true }).tap();
  } else {
    await page.getByText('Network', { exact: true }).click();
  }
  await expect(networkOption).toBeChecked();
  await expect(charts.nth(2)).toBeVisible();
  await expect(charts.nth(1)).toBeHidden();
  await expect(page.locator('.dashboard-chart-view:visible')).toHaveCount(1);
  const networkData = await charts.nth(2).evaluate(element => JSON.parse(element.dataset.chartSeries));
  expect(networkData.series.map(series => series.label)).toEqual(['Receive', 'Transmit']);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

  if (testInfo.project.name === 'chromium') {
    for (const theme of ['light', 'dark']) {
      await page.setViewportSize({ width: 2560, height: 1440 });
      await page.emulateMedia({ colorScheme: theme });
      await page.screenshot({ path: `test-results/dashboard-synthetic-${theme}-2560x1440.png` });
    }
  }
});
