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
  await expect(page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Empty chart' }) }).getByText('No observations.')).toBeVisible();
  await expect(page.getByRole('alert')).toHaveText('Chart data could not be displayed.');
  await expect(page.getByText('42%')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('selection is scoped, keyboard operable and handles disabled rows', async ({ page }, testInfo) => {
  const projects = page.locator('[data-select-list]').filter({ has: page.getByRole('heading', { name: 'Projects', exact: true }) });
  const other = page.locator('[data-select-list]').filter({ has: page.getByRole('heading', { name: 'Other resources' }) });
  const selectAll = projects.getByLabel('Select all Projects');
  await page.evaluate(() => {
    window.selectionEvents = [];
    document.addEventListener('selectionchange', event => window.selectionEvents.push(event.detail.selected));
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
    await expect(page.getByText('No observations.').first()).toBeVisible();
    await expect(page.locator('noscript > .chart-message').first()).toBeVisible();
  } finally {
    await context.close();
  }
});
