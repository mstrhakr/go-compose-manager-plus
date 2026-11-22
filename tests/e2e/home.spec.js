import { test, expect } from '@playwright/test';
import { mkdtemp, mkdir, writeFile, unlink, rmdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

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
  const project = join(root, 'sample');
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
    await expect(page.getByRole('heading', { name: 'Compose projects' })).toBeVisible();
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
    await expect(page.getByText('Available · Inactive')).toBeVisible();
    await page.getByRole('button', { name: 'Activate' }).click();
    await expect(page.getByText('Available · Active')).toBeVisible();
    await expect(page.getByRole('status')).toContainText('No Compose operation was performed');
    await page.getByRole('button', { name: 'Refresh Docker status' }).click();
    await expect(page.getByText('Docker status unavailable: no socket configured.')).toBeVisible();
    await page.goto('/');
    await page.screenshot({ path: 'test-results/auth-dashboard-desktop.png', fullPage: true });

    const themed = await browser.newContext({ baseURL, colorScheme: 'dark', viewport: { width: 1440, height: 900 } });
    try {
      await themed.addCookies(await context.cookies());
      const themedPage = await themed.newPage();
      await themedPage.goto('/');
      await expect(themedPage.getByRole('heading', { name: 'Local instance' })).toBeVisible();
      await expect(themedPage.getByText('Docker integration pending')).toBeVisible();
      await expect(themedPage.getByText('Runtime metrics unavailable')).toBeVisible();
      await expect(themedPage.locator('.stack-tile')).toContainText(project);
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
      await themedPage.screenshot({ path: 'test-results/auth-dashboard-dark.png', fullPage: true });
      await themedPage.getByRole('button', { name: 'Switch to light theme' }).click();
      await themedPage.reload();
      await expect(themedPage.locator('html')).toHaveAttribute('data-theme', 'light');
      await expect(themedPage.getByText('Available · Active')).toBeVisible();
      expect(await themedPage.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    } finally {
      await themed.close();
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
      await expect(touchPage.getByText('Available · Active')).toBeVisible();
      await touchPage.getByRole('button', { name: 'Scan for projects' }).tap();
      await expect(touchPage.getByRole('status')).toContainText('Scan complete');
      await expect(touchPage.getByText('Available · Active')).toBeVisible();
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
      await expect(mobilePage.getByText('Available · Active')).toBeVisible();
      await expect(mobilePage.getByLabel('Discovery root (application-visible path)')).toBeVisible();
      await expect(mobilePage.locator('[data-theme-toggle]')).toBeHidden();
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