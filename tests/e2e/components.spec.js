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
  await expect(chart.locator('.chart-series-details summary')).toHaveText('2 of 2 series');
  await chart.locator('.chart-series-details summary').click();
  await expect(chart.locator('.chart-key').filter({ hasText: 'CPU' })).toBeVisible();
  await chart.getByText('View chart data').click();
  await expect(chart.getByRole('cell', { name: '100', exact: true })).toBeVisible();
  await expect(chart.getByRole('row').filter({ hasText: 'Missing observation' })).toContainText('CPU3Missing observation');
  await expect(page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Empty chart' }) }).getByText('No observations.')).toBeVisible();
  await expect(page.getByRole('alert')).toHaveText('Chart data could not be displayed.');
  await expect(page.getByText('42%')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('keeps large chart legends bounded and lets viewers isolate series', async ({ page }, testInfo) => {
  const chart = page.locator('.chart-panel').filter({ has: page.getByRole('heading', { name: 'Activity', exact: true }) });
  await chart.locator('[data-chart]').evaluate(element => {
    element.dataset.chartSeries = JSON.stringify({
      min: 0, max: 100,
      series: Array.from({ length: 40 }, (_, index) => ({
        label: `Container ${index}`,
        color: ['blue', 'amber', 'purple', 'green', 'red'][index % 5],
        points: [{ x: 1, y: index }, { x: 2, y: index + 1 }],
      })),
    });
    element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  const details = chart.locator('.chart-series-details');
  await expect(details.locator('summary')).toHaveText('40 of 40 series');
  const chartHeight = await chart.locator('[data-chart]').evaluate(element => element.getBoundingClientRect().height);
  await details.locator('summary').click();
  const list = chart.locator('.chart-series-list');
  await expect(list.getByRole('button', { name: 'Container 0' })).toBeVisible();
  expect(await list.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true);
  expect(await chart.locator('[data-chart]').evaluate(element => element.getBoundingClientRect().height)).toBe(chartHeight);
  await list.getByRole('button', { name: 'Container 0' }).click();
  await expect(details.locator('summary')).toHaveText('39 of 40 series');
  await expect(chart.locator('polyline[data-series-index="0"]')).toHaveCSS('display', 'none');
  await details.locator('summary').click();
  if (testInfo.project.name === 'mobile-chromium') {
    await chart.locator('svg').tap({ position: { x: 300, y: 40 } });
  } else {
    await chart.locator('svg').hover({ position: { x: 300, y: 40 } });
  }
  await expect(chart.locator('.chart-tooltip')).not.toContainText('Container 0:');
  await expect(chart.locator('.chart-tooltip')).toContainText('Container 1:');
  await page.getByRole('heading', { name: 'Component test fixture' }).click();
  await expect(chart.locator('.chart-tooltip')).toBeHidden();
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

test('timeline markers and keyboard selection stay synchronized with every chart', async ({ page }, testInfo) => {
  const charts = page.locator('[data-chart]');
  for (const chart of await charts.all()) {
    await chart.evaluate(element => {
      element.dataset.chartSeries = JSON.stringify({
        min: 0, max: 10, xMin: 1000, xMax: 4000, selectedTime: 4000,
        series: [{ label: 'CPU', color: 'blue', points: [
          { x: 1000, y: 1 }, { x: 2000, y: 3 }, { x: 3000, y: 5 },
          { x: 3750, y: 7 }, { x: 4000, y: 8 },
        ] }],
      });
      element.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
    });
  }
  await page.evaluate(() => {
    const timeline = document.createElement('div');
    timeline.className = 'observability-timeline';
    timeline.dataset.timeline = '';
    timeline.dataset.start = '1000';
    timeline.dataset.end = '4000';
    timeline.dataset.selected = '4000';
    timeline.innerHTML = `<section class="timeline-panel">
      <label for="timeline-fixture">Selected time <output data-timeline-output for="timeline-fixture">Initial</output></label>
      <input id="timeline-fixture" type="range" min="1000" max="4000" step="1000" value="4000" data-timeline-slider aria-label="Selected time in the last hour">
      <button type="button" data-timeline-marker data-timeline-select="2000" data-position="33.333" aria-label="Start event"></button>
    </section>
    <section class="timeline-events">
      <button type="button" class="timeline-event" data-timeline-select="2000" aria-label="Select event at 2000">Start event</button>
      <button type="button" class="timeline-event" data-timeline-select="3750" aria-label="Select event at 3750">Restart event</button>
    </section>`;
    document.body.append(timeline);
    timeline.dispatchEvent(new CustomEvent('htmx:afterSwap', { bubbles: true }));
  });
  const timeline = page.locator('[data-timeline]');
  const slider = timeline.getByLabel('Selected time in the last hour');
  await expect(slider).toHaveValue('4000');
  expect(await timeline.locator('[data-timeline-marker]').evaluate(element => element.style.left)).toBe('33.333%');

  if (testInfo.project.name === 'mobile-chromium') {
    await timeline.getByRole('button', { name: 'Start event' }).tap();
  } else {
    await timeline.getByRole('button', { name: 'Start event' }).click();
  }
  await expect(slider).toHaveValue('2000');
  await expect(timeline.getByRole('button', { name: 'Select event at 2000' })).toHaveAttribute('aria-current', 'time');
  const cursors = await charts.evaluateAll(elements => elements.map(element => element.querySelector('.chart-cursor').getAttribute('x1')));
  expect(new Set(cursors).size).toBe(1);

  const restartEvent = timeline.getByRole('button', { name: 'Select event at 3750' });
  if (testInfo.project.name === 'mobile-chromium') await restartEvent.tap();
  else await restartEvent.click();
  await expect(slider).toHaveValue('4000');
  await expect(timeline).toHaveAttribute('data-selected', '3750');
  await expect(timeline.getByRole('button', { name: 'Select event at 3750' })).toHaveAttribute('aria-current', 'time');
  await expect(charts.nth(0).locator('svg')).toHaveAttribute('aria-valuenow', '3750');
  await charts.nth(0).locator('svg').focus();
  await page.keyboard.press('ArrowLeft');
  await expect(timeline).toHaveAttribute('data-selected', '3000');

  if (testInfo.project.name === 'mobile-chromium') await restartEvent.tap();
  else await restartEvent.click();
  await expect(timeline).toHaveAttribute('data-selected', '3750');
  await slider.focus();
  await page.keyboard.press('ArrowLeft');
  await expect(slider).toHaveValue('3000');
  await expect(timeline).toHaveAttribute('data-selected', '3000');

  const svg = charts.nth(0).locator('svg');
  const bounds = await svg.boundingBox();
  const chartPoint = { x: bounds.width * 2 / 3, y: bounds.height / 2 };
  if (testInfo.project.name === 'mobile-chromium') {
    await svg.tap({ position: chartPoint });
  } else {
    await svg.click({ position: chartPoint });
  }
  await expect(slider).toHaveValue('3000');
  const chartTime = await svg.getAttribute('aria-valuenow');
  expect(Number(chartTime)).toBeGreaterThanOrEqual(2997);
  expect(Number(chartTime)).toBeLessThanOrEqual(3000);
  await expect(timeline).toHaveAttribute('data-selected', chartTime);
  await slider.focus();
  await page.keyboard.press('ArrowLeft');
  await expect(slider).toHaveValue('2000');
  expect(await charts.nth(1).locator('svg').getAttribute('aria-valuenow')).toBe('2000');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
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
