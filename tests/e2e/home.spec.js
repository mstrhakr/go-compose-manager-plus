import { test, expect } from '@playwright/test';
import { mkdtemp, mkdir, writeFile, unlink, rmdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';

test('unauthenticated home opens the login view without horizontal overflow', async ({ page }) => {
  await page.goto('/');

  await expect(page).toHaveURL(/\/login$/);
  await expect(page).toHaveTitle('Log in · Compose Manager Plus');
  await expect(page.getByRole('heading', { level: 1, name: 'Welcome back' })).toBeVisible();
  await expect(page.getByLabel('Username')).toBeVisible();
  await expect(page.getByLabel('Password')).toBeVisible();
  await expect(page.getByRole('button', { name: /Sign in/ })).toBeEnabled();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('keyboard can reach username, password, and sign in controls', async ({ page }) => {
  await page.goto('/login');
  await expect(page.getByLabel('Username')).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.getByLabel('Password')).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('button', { name: /Sign in/ })).toBeFocused();
});

test('theme follows the system until selected and persists across navigation', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto('/login');
  const toggle = page.getByRole('button', { name: 'Switch to light theme' });
  await expect(toggle).toBeVisible();
  await expect(page.locator('html')).not.toHaveAttribute('data-theme');
  const darkBackground = await page.locator('body').evaluate(element => getComputedStyle(element).backgroundColor);
  await toggle.focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  expect(await page.locator('body').evaluate(element => getComputedStyle(element).backgroundColor)).not.toBe(darkBackground);
  await expect(page.getByRole('button', { name: 'Switch to dark theme' })).toBeVisible();
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await page.emulateMedia({ colorScheme: 'light' });
  await page.getByRole('button', { name: 'Switch to dark theme' }).click();
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
});

test('unavailable browser storage is reported and theme remains usable', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', {
      get() { throw new DOMException('Storage blocked', 'SecurityError'); },
    });
  });
  await page.goto('/login');
  await expect(page.getByRole('status')).toContainText('Theme preference could not be saved');
  const toggle = page.locator('[data-theme-toggle]');
  await toggle.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', /^(light|dark)$/);
});

test('first login forces password setup before protected dashboard and works on mobile', async ({ browser, baseURL }, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium', 'Single bootstrap account is exercised once to avoid parallel setup races.');
  const root = await mkdtemp(join(tmpdir(), 'compose-manager-discovery-'));
  const project = join(root, `sample-${'long-name-'.repeat(8)}`);
  const compose = join(project, 'compose.yaml');
  await mkdir(project);
  await writeFile(compose, 'services:\n  web:\n    image: example\n');
  const context = await browser.newContext({ javaScriptEnabled: false, baseURL, viewport: { width: 1440, height: 900 } });
  try {
    const page = await context.newPage();
    let loginOrigin = '';
    let loginReferer = '';
    page.on('request', request => {
      if (request.method() === 'POST' && request.url().endsWith('/login')) {
        loginOrigin = request.headers().origin ?? '';
        loginReferer = request.headers().referer ?? '';
      }
    });
    await page.goto('/login');
    await page.getByLabel('Username').fill('playwright-admin');
    await page.getByLabel('Password').fill('playwright-temporary-bootstrap-password');
    await page.getByRole('button', { name: /Sign in/ }).click();
    expect({ loginOrigin, loginReferer }).toEqual({ loginOrigin: baseURL, loginReferer: `${baseURL}/login` });
    await expect(page).toHaveURL(/\/setup\/password$/);
    await expect(page.getByRole('heading', { name: 'Secure your account' })).toBeVisible();
    await page.getByLabel('New password', { exact: true }).fill('playwright-new-password-456');
    await page.getByLabel('Confirm new password', { exact: true }).fill('playwright-new-password-456');
    await page.getByRole('button', { name: /Save password/ }).click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole('heading', { name: 'System status' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Overview views' }).getByRole('link', { name: 'Dashboard' })).toHaveAttribute('aria-current', 'page');
    for (const name of ['Resources unavailable', 'Activity unavailable']) {
      await expect(page.getByRole('button', { name })).toBeDisabled();
    }
    for (const [metric, name] of [['CPU', 'CPU across collected containers'], ['Memory', 'Raw memory across collected containers'], ['Network', 'Network throughput across collected containers']]) {
      await page.locator('.dashboard-chart-options').getByText(metric, { exact: true }).click();
      await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
    }
    await page.locator('.dashboard-chart-options').getByText('CPU', { exact: true }).click();
    await expect(page.getByText('Host-level resource metrics are not collected', { exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'CPU by container', exact: true })).toHaveCount(0);
    await expect(page.getByText('No metrics are being collected.', { exact: false }).first()).toBeVisible();
    await page.goto('/containers/observability');
    await expect(page.getByRole('heading', { level: 1, name: 'Container observability' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Container views' }).getByRole('link', { name: 'Observability' })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('slider', { name: 'Selected time in the last hour' })).toBeDisabled();
    await expect(page.locator('[data-timeline-message]')).toContainText('State and event history is Off');
    for (const name of ['CPU usage', 'Memory usage (Docker raw)', 'Network throughput']) {
      await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Observability settings' })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Settings categories' }).getByRole('link', { name: 'Containers' })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByText('Disabled; no background metrics collection', { exact: false })).toBeVisible();
    await expect(page.locator('[data-metrics-custom-options]')).toBeHidden();
    await expect(page.locator('#metrics-interval')).toHaveValue('15s');
    await expect(page.locator('#metrics-interval')).toHaveAttribute('readonly', '');
    await expect(page.getByLabel('Metrics storage unit')).toHaveValue('MB');
    const storageBounds = await page.getByLabel('Metrics/state storage target').boundingBox();
    const unitBounds = await page.getByLabel('Metrics storage unit').boundingBox();
    expect(unitBounds.x).toBeGreaterThan(storageBounds.x + storageBounds.width - unitBounds.width - 12);
    expect(unitBounds.y + unitBounds.height / 2).toBeGreaterThan(storageBounds.y);
    expect(unitBounds.y + unitBounds.height / 2).toBeLessThan(storageBounds.y + storageBounds.height);
    const interactiveContext = await browser.newContext({ baseURL });
    try {
      await interactiveContext.addCookies(await context.cookies());
      const interactivePage = await interactiveContext.newPage();
      await interactivePage.goto('/containers/observability');
      const timeline = interactivePage.locator('[data-timeline]');
      const eventTime = await timeline.evaluate(element => {
        const selected = Number(element.dataset.start) + 3750;
        const marker = document.createElement('button');
        marker.type = 'button';
        marker.className = 'timeline-marker';
        marker.dataset.timelineSelect = String(selected);
        marker.dataset.timelineMarker = '';
        marker.dataset.position = '1';
        marker.setAttribute('aria-label', 'Restart marker');
        element.querySelector('.timeline-track').append(marker);
        const empty = element.querySelector('.timeline-empty');
        empty?.remove();
        const list = document.createElement('ol');
        list.className = 'timeline-event-list';
        const item = document.createElement('li');
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'timeline-event';
        button.dataset.timelineSelect = String(selected);
        button.setAttribute('aria-label', 'Select rendered restart event');
        button.textContent = 'Restarted';
        item.append(button);
        list.append(item);
        element.querySelector('.timeline-events').append(list);
        return selected;
      });
      await timeline.getByRole('button', { name: 'Restart marker' }).click();
      await expect(timeline.getByRole('button', { name: 'Select rendered restart event' })).toHaveAttribute('aria-current', 'time');
      await expect(timeline).toHaveAttribute('data-selected', String(eventTime));
      await timeline.getByRole('button', { name: 'Select rendered restart event' }).click();
      await expect(timeline.getByRole('slider', { name: 'Selected time in the last hour' })).toHaveValue(String(eventTime + 250));
      await expect(timeline).toHaveAttribute('data-selected', String(eventTime));

      await interactivePage.goto('/settings');
      const stats = interactivePage.getByRole('group', { name: 'Container stats' });
      await stats.getByRole('radio', { name: 'Low', exact: true }).press('Space');
      await expect(interactivePage.getByLabel('Collection scope')).toBeHidden();
      await expect(interactivePage.locator('#metrics-interval')).toHaveJSProperty('readOnly', true);
      await stats.getByRole('radio', { name: 'Custom', exact: true }).press('Space');
      await expect(interactivePage.locator('[data-metrics-custom-options]')).toBeVisible();
      await interactivePage.getByLabel('Collection scope').selectOption('all');
      await stats.getByRole('radio', { name: 'High', exact: true }).press('Space');
      await expect(interactivePage.getByLabel('Collection scope')).toBeHidden();
      await expect(interactivePage.getByLabel('Collection scope')).toHaveValue('all');
      await expect(interactivePage.locator('#metrics-interval')).toHaveValue('1s');
      await expect(interactivePage.locator('#metrics-interval')).toHaveJSProperty('readOnly', true);
      await interactivePage.getByLabel('Metrics/state storage target').fill('10.48576');
      await interactivePage.getByLabel('Metrics storage unit').selectOption('MB');
      await expect(stats.getByRole('radio', { name: 'Custom', exact: true })).toBeChecked();
      await expect(interactivePage.locator('[data-metrics-custom-options]')).toBeVisible();
    } finally {
      await interactiveContext.close();
    }
    await page.getByLabel('Metrics/state storage target').fill('10.48576');
    await page.getByLabel('Metrics storage unit').selectOption('MB');
    await page.getByRole('button', { name: 'Save observability preferences' }).click();
    await expect(page.getByRole('status').filter({ hasText: 'Observability preferences saved' })).toBeVisible();
    const stats = page.getByRole('group', { name: 'Container stats' });
    await expect(stats.getByRole('radio', { name: 'Off', exact: true })).toBeChecked();
    await expect(page.getByLabel('Metrics/state storage target')).toHaveValue('10.48576');
    await page.getByRole('group', { name: 'Overall collection profile' }).getByRole('radio', { name: 'Custom', exact: true }).press('Space');
    await page.getByRole('button', { name: 'Save observability preferences' }).click();
    await expect(page.getByRole('status').filter({ hasText: 'Observability preferences saved' })).toBeVisible();
    await stats.getByRole('radio', { name: 'Medium', exact: true }).press('Space');
    await page.getByRole('button', { name: 'Save observability preferences' }).click();
    await expect(page.getByRole('alert')).toContainText('Configure a Docker socket');
    await stats.getByRole('radio', { name: 'Off', exact: true }).press('Space');
    await page.setViewportSize({ width: 375, height: 812 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.getByLabel('Log storage target').fill('300');
    await page.getByLabel('Log storage unit').selectOption('MB');
    await page.getByRole('button', { name: 'Save observability preferences' }).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('status').filter({ hasText: 'Observability preferences saved' })).toBeVisible();
    await expect(page.getByLabel('Log storage target')).toHaveValue('300');
    const touchContext = await browser.newContext({ baseURL, viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true });
    try {
      await touchContext.addCookies(await context.cookies());
      const touchPage = await touchContext.newPage();
      await touchPage.goto('/settings');
      await expect(touchPage.getByRole('group', { name: 'Container stats' }).getByRole('radio', { name: 'Off', exact: true })).toBeChecked();
      await touchPage.getByLabel('Metrics/state storage target').tap({ position: { x: 20, y: 20 } });
      await expect(touchPage.getByLabel('Metrics/state storage target')).toBeFocused();
      expect(await touchPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    } finally {
      await touchContext.close();
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.getByRole('link', { name: 'Overview', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Discovered stacks' })).toBeVisible();
    await expect(page.getByText('Docker integration pending')).toBeVisible();
    await page.getByLabel('Discovery root (application-visible path)').fill('relative-path');
    await page.getByLabel('Discovery root (application-visible path)').focus();
    await page.keyboard.press('Tab');
    await expect(page.getByRole('button', { name: 'Save root' })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('alert')).toContainText('Enter an absolute application-visible discovery root');
    await page.getByLabel('Discovery root (application-visible path)').fill(root);
    await page.getByRole('button', { name: 'Save root' }).click();
    await page.getByRole('button', { name: 'Scan for projects' }).click();
    await expect(page.getByText('1 project discovered')).toBeVisible();
    await page.getByRole('link', { name: 'Compose projects', exact: true }).click();
    await expect(page).toHaveURL(/\/projects$/);
    const projectViews = page.getByRole('navigation', { name: 'Project views' });
    await expect(projectViews.getByRole('link', { name: 'Projects' })).toHaveAttribute('aria-current', 'page');
    await expect(projectViews.getByRole('button', { name: 'Activity' })).toBeDisabled();
    await page.getByLabel('Search', { exact: true }).fill('no-such-project');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page.getByText('No projects match these filters.')).toBeVisible();
    await page.getByRole('link', { name: 'Clear filters' }).click();
    const projectRow = page.getByRole('row').filter({ hasText: basename(project) });
    await projectRow.getByRole('link', { name: basename(project), exact: true }).focus();
    await page.keyboard.press('Enter');
    const projectDetails = page.getByRole('dialog', { name: basename(project), exact: true });
    await expect(projectDetails).toBeVisible();
    await expect(projectDetails).toContainText('Docker status unavailable');
    await projectDetails.getByRole('button', { name: 'Activate' }).click();
    await expect(page).toHaveURL(/\/projects$/);
    await expect(page.getByRole('status')).toContainText('No Compose operation was performed');
    await expect(projectRow).toContainText('Active');
    await page.getByRole('link', { name: 'Overview', exact: true }).click();
    await page.getByRole('button', { name: 'Refresh Docker status' }).click();
    await expect(page.getByText('Docker status unavailable: no socket configured.').first()).toBeVisible();
    await page.getByRole('link', { name: 'Containers', exact: true }).click();
    await expect(page).toHaveURL(/\/containers$/);
    const containerViews = page.getByRole('navigation', { name: 'Container views' });
    await expect(containerViews.getByRole('link', { name: 'Containers' })).toHaveAttribute('aria-current', 'page');
    await expect(containerViews.getByRole('link', { name: 'Observability' })).toBeVisible();
    await containerViews.getByRole('link', { name: 'Observability' }).click();
    await expect(page).toHaveURL(/\/containers\/observability$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Container observability' })).toBeVisible();
    await page.getByRole('navigation', { name: 'Container views' }).getByRole('link', { name: 'Containers' }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'Containers' })).toBeVisible();
    await expect(page.locator('.list-empty')).toContainText('Docker status unavailable');
    await expect(page.locator('.list-empty')).not.toContainText('No containers found');
    await page.goto('/');
    await page.screenshot({ path: 'test-results/auth-dashboard-desktop.png', fullPage: true });

    const themed = await browser.newContext({ baseURL, colorScheme: 'dark', viewport: { width: 2560, height: 1440 } });
    try {
      await themed.addCookies(await context.cookies());
      const themedPage = await themed.newPage();
      await themedPage.goto('/');
      await expect(themedPage.getByRole('heading', { name: 'Local instance' })).toBeVisible();
      await expect(themedPage.getByText('Docker integration pending')).toBeVisible();
      await expect(themedPage.getByText('No metrics are being collected.', { exact: false }).first()).toBeVisible();
      await expect(themedPage.getByText('1 project discovered')).toBeVisible();
      await expect(themedPage.getByRole('link', { name: 'View project details' })).toHaveAttribute('href', '/projects');
      const layout = await themedPage.evaluate(() => {
        const bounds = selector => document.querySelector(selector).getBoundingClientRect();
        return {
          header: bounds('.console-header').height,
          rail: bounds('.console-rail').width,
          sidebar: bounds('.host-sidebar').width,
          chart: bounds('.chart').height,
          sidebarRight: bounds('.host-sidebar').right,
          mainLeft: bounds('.console-main').left,
        };
      });
      expect(layout.header).toBeGreaterThanOrEqual(44);
      expect(layout.header).toBeLessThanOrEqual(45);
      expect(layout.rail).toBe(48);
      expect(layout.sidebar).toBe(272);
      expect(layout.chart).toBe(245);
      expect(layout.mainLeft).toBe(layout.sidebarRight);
      expect(await themedPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      for (const width of [1440, 1024, 901, 900, 621, 620]) {
        await themedPage.setViewportSize({ width, height: 900 });
        expect(await themedPage.evaluate(() => {
          const charts = [...document.querySelectorAll('.chart-panel')].filter(chart => chart.getBoundingClientRect().width > 0);
          return document.documentElement.scrollWidth <= window.innerWidth &&
            charts.length === 1;
        })).toBe(true);
      }
      await themedPage.setViewportSize({ width: 1440, height: 520 });
      const scrollBehavior = await themedPage.evaluate(() => {
        const main = document.querySelector('.console-main');
        const sidebar = document.querySelector('.host-sidebar');
        const rail = document.querySelector('.console-rail');
        const sidebarTop = sidebar.getBoundingClientRect().top;
        const railTop = rail.getBoundingClientRect().top;
        main.scrollTop = main.scrollHeight;
        const sidebarContent = sidebar.querySelector('.sidebar-content');
        sidebarContent.scrollTop = sidebarContent.scrollHeight;
        return {
          documentFits: document.documentElement.scrollHeight <= window.innerHeight,
          mainScrolls: main.scrollTop > 0,
          sidebarScrolls: sidebarContent.scrollTop > 0,
          windowStaysPut: window.scrollY === 0,
          sidebarStaysPut: sidebar.getBoundingClientRect().top === sidebarTop,
          railStaysPut: rail.getBoundingClientRect().top === railTop,
        };
      });
      expect(scrollBehavior).toEqual({
        documentFits: true,
        mainScrolls: true,
        sidebarScrolls: true,
        windowStaysPut: true,
        sidebarStaysPut: true,
        railStaysPut: true,
      });
      const mainPane = themedPage.locator('.console-main');
      const sidebarPane = themedPage.getByRole('region', { name: 'Host panel content' });
      await mainPane.evaluate(element => { element.scrollTop = 0; });
      await mainPane.focus();
      await themedPage.keyboard.press('PageDown');
      await expect.poll(() => mainPane.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
      await sidebarPane.evaluate(element => { element.scrollTop = 0; });
      await sidebarPane.hover();
      await themedPage.mouse.wheel(0, 300);
      await expect.poll(() => sidebarPane.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
      expect(await themedPage.evaluate(() => window.scrollY)).toBe(0);
      await themedPage.goto('/settings');
      const settingsNav = themedPage.getByRole('navigation', { name: 'Settings categories' });
      const settingsTop = (await settingsNav.boundingBox()).y;
      const settingsMain = themedPage.locator('.console-main');
      await settingsMain.evaluate(element => { element.scrollTop = element.scrollHeight; });
      expect(await settingsMain.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
      expect((await settingsNav.boundingBox()).y).toBeCloseTo(settingsTop, 0);
      await themedPage.goto('/');
      await themedPage.setViewportSize({ width: 2560, height: 1440 });
      await themedPage.screenshot({ path: 'test-results/auth-dashboard-dark.png', fullPage: true });
      const hostPanel = themedPage.locator('.sidebar-disclosure');
      await hostPanel.locator('summary').focus();
      await themedPage.keyboard.press('Enter');
      await expect(hostPanel).not.toHaveAttribute('open', '');
      await expect(themedPage.getByLabel('Discovery root (application-visible path)')).toBeHidden();
      await themedPage.keyboard.press('Enter');
      await expect(themedPage.getByLabel('Discovery root (application-visible path)')).toBeVisible();
      await themedPage.getByRole('button', { name: 'Expand navigation' }).focus();
      await themedPage.keyboard.press('Enter');
      await expect(themedPage.getByRole('button', { name: 'Collapse navigation' })).toHaveAttribute('aria-expanded', 'true');
      await themedPage.getByRole('link', { name: 'Compose projects', exact: true }).click();
      await expect(themedPage.getByRole('button', { name: 'Collapse navigation' })).toBeVisible();
      const projectColumns = themedPage.locator('[data-column-manager="projects-title"]');
      await projectColumns.locator('[data-column-controls] summary').click();
      await projectColumns.getByLabel('Activation', { exact: true }).uncheck();
      await expect(projectColumns.locator('tbody [data-column="Activation"]')).toBeHidden();
      await themedPage.reload();
      await expect(projectColumns.locator('tbody [data-column="Activation"]')).toBeHidden();
      await projectColumns.locator('[data-column-controls] summary').click();
      await projectColumns.getByRole('button', { name: 'Restore defaults' }).click();
      await expect(projectColumns.locator('tbody [data-column="Activation"]')).toBeVisible();
      await projectColumns.locator('[data-column-controls] summary').click();
      const filters = themedPage.locator('.inventory-filter-disclosure');
      for (const width of [1024, 901, 621]) {
        await themedPage.setViewportSize({ width, height: 900 });
        expect(await themedPage.evaluate(() => {
          const panel = document.querySelector('.inventory-panel').getBoundingClientRect();
          const elements = document.querySelectorAll('.inventory-panel .list-scroll');
          return document.documentElement.scrollWidth <= window.innerWidth &&
            [...elements].every(element => {
              const rect = element.getBoundingClientRect();
              return rect.left >= panel.left && rect.right <= panel.right;
            });
        })).toBe(true);
      }
      await themedPage.setViewportSize({ width: 1440, height: 900 });
      await filters.locator('summary').click();
      await expect(filters).not.toHaveAttribute('open', '');
      await expect(themedPage.getByLabel('Search', { exact: true })).toBeHidden();
      await filters.locator('summary').click();
      await expect(themedPage.getByLabel('Search', { exact: true })).toBeVisible();
      await themedPage.getByRole('button', { name: 'Collapse navigation' }).click();
      await themedPage.goto('/');
      await themedPage.getByRole('button', { name: 'Switch to light theme' }).click();
      await themedPage.reload();
      await expect(themedPage.locator('html')).toHaveAttribute('data-theme', 'light');
      await expect(themedPage.getByText('1 project discovered')).toBeVisible();
      expect(await themedPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    } finally {
      await themed.close();
    }

    const blockedStorage = await browser.newContext({ baseURL });
    try {
      await blockedStorage.addCookies(await context.cookies());
      await blockedStorage.addInitScript(() => {
        Object.defineProperty(window, 'localStorage', {
          get() { throw new DOMException('Storage blocked', 'SecurityError'); },
        });
      });
      const blockedPage = await blockedStorage.newPage();
      await blockedPage.goto('/projects');
      await expect(blockedPage.getByText('Navigation preference could not be saved in this browser.')).toBeVisible();
      await blockedPage.getByRole('button', { name: 'Expand navigation' }).click();
      await expect(blockedPage.getByRole('button', { name: 'Collapse navigation' })).toBeVisible();
    } finally {
      await blockedStorage.close();
    }

    const touch = await browser.newContext({
      baseURL, colorScheme: 'dark', isMobile: true, hasTouch: true,
      viewport: { width: 390, height: 844 },
    });
    try {
      await touch.addCookies(await context.cookies());
      const touchPage = await touch.newPage();
      await touchPage.goto('/');
      await touchPage.getByRole('button', { name: 'Switch to light theme' }).tap();
      await touchPage.reload();
      await expect(touchPage.locator('html')).toHaveAttribute('data-theme', 'light');
      await expect(touchPage.getByText('1 project discovered')).toBeVisible();
      await touchPage.getByRole('button', { name: 'Scan for projects' }).tap();
      await expect(touchPage.getByRole('status')).toContainText('Scan complete');
      await expect(touchPage.getByText('1 project discovered')).toBeVisible();
      await touchPage.getByRole('link', { name: 'Compose projects', exact: true }).tap();
      await touchPage.getByRole('link', { name: 'Filters & sort' }).tap();
      await touchPage.getByText('Collapse filters', { exact: true }).tap();
      await expect(touchPage.getByLabel('Search', { exact: true })).toBeHidden();
      await touchPage.getByRole('link', { name: 'Filters & sort' }).tap();
      await expect(touchPage.getByLabel('Search', { exact: true })).toBeVisible();
      await touchPage.getByLabel('Search', { exact: true }).fill('no-such-project');
      await touchPage.getByRole('button', { name: 'Apply filters' }).tap();
      await expect(touchPage.getByText('No projects match these filters.')).toBeVisible();
      await touchPage.getByRole('link', { name: 'Clear filters' }).tap();
      const touchRow = touchPage.getByRole('row').filter({ hasText: basename(project) });
      await touchRow.getByRole('link', { name: basename(project), exact: true }).tap();
      const touchDetails = touchPage.getByRole('dialog', { name: basename(project), exact: true });
      await expect(touchDetails).toBeVisible();
      expect(await touchPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await touchDetails.getByRole('link', { name: 'Close details' }).tap();
      await touchPage.getByRole('link', { name: 'Containers', exact: true }).tap();
      await expect(touchPage.locator('.list-empty')).toContainText('Docker status unavailable');
      await touchPage.getByRole('navigation', { name: 'Container views' }).getByRole('link', { name: 'Observability' }).tap();
      const timelineSlider = touchPage.getByRole('slider', { name: 'Selected time in the last hour' });
      await expect(timelineSlider).toBeVisible();
      const endTime = Number(await timelineSlider.getAttribute('max'));
      const timelineViewport = touchPage.locator('[data-timeline-viewport]');
      const timelineBounds = await timelineViewport.boundingBox();
      await timelineViewport.tap({ position: { x: 4, y: timelineBounds.height / 2 } });
      await expect.poll(async () => Number(await timelineSlider.inputValue())).toBeLessThan(endTime);
      expect(await touchPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      expect(await touchPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    } finally {
      await touch.close();
    }

    const mobile = await browser.newContext({
      baseURL,
      javaScriptEnabled: false,
      isMobile: true,
      hasTouch: true,
      viewport: { width: 390, height: 844 },
    });
    try {
      await mobile.addCookies(await context.cookies());
      const mobilePage = await mobile.newPage();
      await mobilePage.goto('/');
      await expect(mobilePage.getByRole('heading', { name: 'System status' })).toBeVisible();
      await expect(mobilePage.getByText('1 project discovered')).toBeVisible();
      await expect(mobilePage.getByLabel('Discovery root (application-visible path)')).toBeVisible();
      await expect(mobilePage.locator('[data-theme-toggle]')).toBeHidden();
      await mobilePage.getByText('Collapse host panel', { exact: true }).tap();
      await expect(mobilePage.getByLabel('Discovery root (application-visible path)')).toBeHidden();
      await mobilePage.getByText('Expand host panel', { exact: true }).tap();
      await mobilePage.getByRole('link', { name: 'Compose projects', exact: true }).tap();
      await mobilePage.getByText('Collapse filters', { exact: true }).tap();
      await expect(mobilePage.getByLabel('Search', { exact: true })).toBeHidden();
      await mobilePage.getByText('Expand filters', { exact: true }).tap();
      await expect(mobilePage.getByLabel('Search', { exact: true })).toBeVisible();
      expect(await mobilePage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await mobilePage.screenshot({ path: 'test-results/auth-dashboard-mobile.png', fullPage: true });
    } finally {
      await mobile.close();
    }
  } finally {
    await context.close();
    await unlink(compose);
    await rmdir(project);
    await rmdir(root);
  }
});