import { execFileSync } from 'node:child_process';
import { test, expect } from '@playwright/test';

const fixture = execFileSync('go', ['run', './tests/ui-fixture'], { encoding: 'utf8' });

test.beforeEach(async ({ page }) => {
  await page.route('**/component-test-fixture', route => route.fulfill({
    status: 200, contentType: 'text/html', body: fixture,
  }));
  await page.goto('/component-test-fixture');
});

test('renders chart series, gaps, data table and explicit empty/error states', async ({ page }) => {
  const chart = page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Activity', exact: true }) });
  await expect(chart.locator('polyline')).toHaveCount(2);
  await expect(chart.locator('circle')).toHaveCount(1);
  await expect(chart.locator('.chart-key').filter({ hasText: 'CPU' })).toBeVisible();
  await chart.getByText('View chart data').click();
  await expect(chart.getByRole('cell', { name: '100', exact: true })).toBeVisible();
  await expect(chart.getByRole('row').filter({ hasText: 'Missing observation' })).toContainText('CPU3Missing observation');
  await expect(page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Empty chart' }) }).getByText('No observations.')).toBeVisible();
  await expect(page.getByRole('alert')).toHaveText('Chart data could not be displayed.');
  await expect(page.getByText('42%')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('charts expose hover values and synchronize pointer and keyboard time selection', async ({ page }, testInfo) => {
  const first = page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Activity', exact: true }) });
  const second = page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Empty chart' }) });
  await second.locator('[data-chart]').evaluate(element => {
    element.dataset.chartSeries = JSON.stringify({
      min: 0, max: 100, xMin: 1, xMax: 4, selectedTime: 4,
      series: [{ label: 'Second chart', color: 'green', points: [{ x: 1, y: 10 }, { x: 4, y: 90 }] }],
    });
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await page.evaluate(() => {
    window.chartTimes = [];
    document.addEventListener('observability:timechange', event => window.chartTimes.push(event.detail.time));
  });
  const svg = first.locator('svg');
  await svg.hover({ position: { x: 200, y: 40 } });
  await expect(first.locator('.chart-tooltip')).toContainText('CPU:');
  await expect(first.locator('.chart-tooltip')).toContainText('Memory:');
  const bounds = await svg.boundingBox();
  const center = { x: bounds.width / 2, y: bounds.height / 2 };
  if (testInfo.project.name === 'mobile-chromium') {
    await svg.tap({ position: center });
  } else {
    await svg.click({ position: center });
  }
  await expect.poll(() => page.evaluate(() => window.chartTimes.length)).toBeGreaterThan(0);
  const firstCursor = first.locator('.chart-cursor');
  const secondCursor = second.locator('.chart-cursor');
  await expect(firstCursor).toHaveAttribute('x1', await secondCursor.getAttribute('x1'));
  await page.evaluate(() => document.dispatchEvent(new CustomEvent('observability:timechange', { detail: { time: 3 } })));
  await expect(firstCursor).toHaveAttribute('x1', await secondCursor.getAttribute('x1'));
  await svg.focus();
  await page.keyboard.press('ArrowLeft');
  await expect.poll(() => page.evaluate(() => window.chartTimes.length)).toBeGreaterThan(1);
  await expect(firstCursor).toHaveAttribute('x1', await secondCursor.getAttribute('x1'));
});

test('selection is scoped, keyboard operable and handles disabled rows', async ({ page }, testInfo) => {
  const projects = page.locator('[data-select-list]').filter({ has: page.getByRole('heading', { name: 'Projects', exact: true }) });
  const other = page.locator('[data-select-list]').filter({ has: page.getByRole('heading', { name: 'Other resources' }) });
  const selectAll = projects.getByLabel('Select all Projects');
  await page.evaluate(() => {
    window.selectionEvents = [];
    document.addEventListener('list:selectionchange', event => window.selectionEvents.push(event.detail.selected));
    document.dispatchEvent(new Event('selectionchange'));
  });
  await projects.getByLabel('Select Alpha').focus();
  await page.keyboard.press('Space');
  await expect(projects.getByText('1 selected')).toBeVisible();
  expect(await selectAll.evaluate(input => input.indeterminate)).toBe(true);
  await selectAll.check();
  await expect(projects.getByText('2 selected')).toBeVisible();
  await expect(projects.getByLabel('Select Locked')).not.toBeChecked();
  await expect(other.getByLabel('Select Other')).not.toBeChecked();
  expect(await page.evaluate(() => window.selectionEvents.at(-1))).toEqual(['alpha', 'beta']);
  await selectAll.uncheck();
  await expect(projects.getByText('0 selected')).toBeVisible();
  if (testInfo.project.name === 'mobile-chromium') {
    await projects.getByLabel('Select Beta').tap();
    await expect(projects.getByText('1 selected')).toBeVisible();
  }
});

test('column preferences apply to headings and cells, persist and restore', async ({ page }, testInfo) => {
  const list = page.locator('[data-column-manager="projects-title"]');
  const editor = list.locator('[data-column-controls]');
  await editor.locator('summary').focus();
  await page.keyboard.press('Enter');
  await expect(editor.getByLabel('Name', { exact: true })).toBeDisabled();
  if (testInfo.project.name === 'mobile-chromium') {
    await editor.getByLabel('Status', { exact: true }).tap();
  } else {
    await editor.getByLabel('Status', { exact: true }).uncheck();
  }
  await expect(list.locator('[data-column="Status"]')).toHaveCount(4);
  for (const cell of await list.locator('[data-column="Status"]').all()) await expect(cell).toBeHidden();
  await expect(page.locator('[data-column-manager="other-title"]').getByRole('columnheader', { name: 'Status' })).toBeVisible();
  await expect(list.getByLabel('Select Alpha')).toBeVisible();
  await page.reload();
  await expect(list.locator('[data-column="Status"]').first()).toBeHidden();
  await list.locator('tbody').evaluate(element => {
    const row = element.firstElementChild.cloneNode(true);
    row.querySelector('[data-column="Status"]').hidden = false;
    element.append(row);
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(list.locator('[data-column="Status"]').last()).toBeHidden();
  await editor.locator('summary').click();
  await editor.getByRole('button', { name: 'Restore defaults' }).click();
  await expect(list.getByRole('columnheader', { name: 'Status' })).toBeVisible();
  await page.reload();
  await expect(list.getByRole('columnheader', { name: 'Status' })).toBeVisible();
});

test('column controls report blocked browser storage but remain usable', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', {
      get() { throw new DOMException('Storage blocked', 'SecurityError'); },
    });
  });
  await page.reload();
  const list = page.locator('[data-column-manager="projects-title"]');
  await list.locator('[data-column-controls] summary').click();
  await expect(list.getByRole('status')).toContainText('Column preferences could not be loaded');
  await list.getByLabel('Status', { exact: true }).uncheck();
  await expect(list.locator('[data-column="Status"]').first()).toBeHidden();
  await expect(list.getByRole('status')).toContainText('Column preferences could not be saved');
  await list.locator('tbody').evaluate(element => element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true })));
  await expect(list.locator('[data-column="Status"]').first()).toBeHidden();
});

test('column controls survive a partial swap of their options and restore button', async ({ page }) => {
  const list = page.locator('[data-column-manager="projects-title"]');
  const editor = list.locator('[data-column-controls]');
  await editor.locator('summary').click();
  await editor.getByLabel('Status', { exact: true }).uncheck();
  await editor.locator('.column-customizer-panel').evaluate(panel => {
    panel.innerHTML = panel.innerHTML;
    panel.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(editor.getByLabel('Status', { exact: true })).not.toBeChecked();
  await editor.getByLabel('Status', { exact: true }).check();
  await expect(list.getByRole('columnheader', { name: 'Status' })).toBeVisible();
  await editor.getByLabel('Status', { exact: true }).uncheck();
  await editor.getByRole('button', { name: 'Restore defaults' }).click();
  await expect(list.getByRole('columnheader', { name: 'Status' })).toBeVisible();
  await page.reload();
  await expect(list.getByRole('columnheader', { name: 'Status' })).toBeVisible();
});

test('reinitializes swapped components and handles zero selectable entries', async ({ page }) => {
  const list = page.locator('[data-select-list]').first();
  await list.evaluate(element => {
    element.querySelectorAll('[data-select-item]').forEach(item => { item.disabled = true; });
    element.querySelector('tbody').dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(list.getByLabel('Select all Projects')).toBeDisabled();
  await expect(list.getByText('0 selected')).toBeVisible();
  const chart = page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Invalid chart' }) });
  await chart.locator('[data-chart]').evaluate(element => {
    element.dataset.chartSeries = JSON.stringify({ min: 0, max: 1, series: [{ label: 'Empty', color: 'blue', points: [{ x: 1, y: null }] }] });
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(chart.getByText('No observations.')).toBeVisible();
  await expect(chart.getByRole('alert')).toBeHidden();
  await chart.getByText('View chart data').click();
  await expect(chart.getByRole('cell', { name: 'Missing observation' })).toBeVisible();
  await chart.locator('[data-chart]').evaluate(element => {
    element.dataset.chartSeries = JSON.stringify({ min: 0, max: 1, series: [{ label: 'Recovered', color: 'blue', points: [{ x: 1, y: 1 }] }] });
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(chart.locator('circle')).toHaveCount(1);
  await chart.locator('[data-chart]').evaluate(element => {
    delete element.dataset.chartSeries;
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  await expect(chart.getByText('No observations.')).toBeVisible();
  await expect(chart.locator('circle')).toHaveCount(0);
});

test('fixture remains usable in both themes and on narrow screens', async ({ page }, testInfo) => {
  for (const theme of ['light', 'dark']) {
    await page.evaluate(value => { document.documentElement.dataset.theme = value; }, theme);
    await expect(page.getByRole('heading', { name: 'Projects', exact: true })).toBeVisible();
    const contrasts = await page.evaluate(() => {
      function rgb(color) {
        const element = document.createElement('span');
        element.style.color = color;
        document.body.append(element);
        const values = getComputedStyle(element).color.match(/\d+/g).slice(0, 3).map(Number);
        element.remove();
        return values;
      }
      function luminance(color) {
        return rgb(color).map(value => value / 255).map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4)
          .reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0);
      }
      function contrast(first, second) {
        const values = [luminance(first), luminance(second)].sort((a, b) => a - b);
        return (values[1] + .05) / (values[0] + .05);
      }
      const style = getComputedStyle(document.documentElement);
      const token = name => style.getPropertyValue(name).trim();
      const label = getComputedStyle(document.querySelector('.chart-key')).color;
      return [
        contrast(token('--dim'), token('--bg')),
        contrast(token('--dim'), token('--panel')),
        contrast(label, token('--sidebar')),
      ];
    });
    for (const ratio of contrasts) expect(ratio).toBeGreaterThanOrEqual(4.5);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: `test-results/components-${testInfo.project.name}-${theme}.png`, fullPage: true });
  }
});

test('native item checkboxes remain usable without JavaScript', async ({ browser, baseURL }) => {
  const context = await browser.newContext({ baseURL, javaScriptEnabled: false });
  try {
    const page = await context.newPage();
    await page.route('**/component-test-fixture', route => route.fulfill({ contentType: 'text/html', body: fixture }));
    await page.goto('/component-test-fixture');
    await page.getByLabel('Select Alpha').check();
    await expect(page.getByLabel('Select Alpha')).toBeChecked();
    await expect(page.getByLabel('Select all Projects')).toBeHidden();
    for (const control of await page.locator('[data-column-controls]').all()) await expect(control).toBeHidden();
    await expect(page.getByText('No observations.').first()).toBeVisible();
    await expect(page.locator('noscript > .chart-message').first()).toBeVisible();
  } finally {
    await context.close();
  }
});
