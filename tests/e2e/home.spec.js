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
    await page.getByLabel('Discovery root (application-visible path)').fill(root);
    await page.getByRole('button', { name: 'Save root' }).click();
    await page.getByRole('button', { name: 'Scan for projects' }).click();
    await expect(page.getByText('Available · Inactive')).toBeVisible();
    await page.getByRole('button', { name: 'Activate' }).click();
    await expect(page.getByText('Available · Active')).toBeVisible();
    await expect(page.getByRole('status')).toContainText('No Compose operation was performed');
    await page.screenshot({ path: 'test-results/auth-dashboard-desktop.png', fullPage: true });

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