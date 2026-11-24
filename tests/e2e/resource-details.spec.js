import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect } from '@playwright/test';

let fixtures;
const firstID = 'a'.repeat(64);
const secondID = 'b'.repeat(64);
const csp = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'";

test.beforeAll(() => {
  mkdirSync('test-results', { recursive: true });
  const directory = mkdtempSync(join(process.cwd(), 'test-results', 'cmp-details-preview-'));
  try {
    execFileSync('go', ['test', './internal/web', '-run', '^TestSyntheticResourceDetailsPreview$', '-count=1'], {
      env: { ...process.env, CMP_UI_PREVIEW_DIR: directory }, stdio: 'pipe',
    });
    fixtures = Object.fromEntries([
      'projects', 'projects-selected', 'containers', 'containers-selected',
      'stack', 'container-a', 'container-b', 'container-long',
    ].map(name => [name, readFileSync(join(directory, `${name}.html`), 'utf8')]));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

function serve(route, override) {
  const request = route.request();
  const url = new URL(request.url());
  const selected = url.searchParams.get('details');
  const fragment = request.headers()['hx-request'] === 'true';
  const projects = url.pathname === '/projects';
  const key = override || (selected ?
    fragment ? projects ? 'stack' : selected === firstID ? 'container-a' : 'container-b' :
      `${projects ? 'projects' : 'containers'}-selected` : projects ? 'projects' : 'containers');
  return route.fulfill({ contentType: 'text/html', headers: { 'Content-Security-Policy': csp }, body: fixtures[key] });
}

async function routeInventory(page) {
  await page.route('**/projects*', route => serve(route));
  await page.route('**/containers*', route => serve(route));
}

test('stack details overlay the shared inventory table without displacement and support keyboard access', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await routeInventory(page);
  const cspErrors = [];
  page.on('console', message => { if (/content.security.policy|refused to execute/i.test(message.text())) cspErrors.push(message.text()); });
  await page.goto('/projects');
  const main = page.locator('main');
  const before = await main.boundingBox();
  const link = page.getByRole('link', { name: 'media-services', exact: true });
  await link.click();
  const dialog = page.getByRole('dialog', { name: 'media-services', exact: true });
  await expect(dialog).toBeVisible();
  expect(await dialog.evaluate(element => element.matches(':modal'))).toBe(false);
  expect(await main.boundingBox()).toEqual(before);
  const bounds = await dialog.boundingBox();
  expect(bounds.x + bounds.width).toBe(1440);
  expect(bounds.width).toBe(400);
  expect(bounds.y).toBe(44);
  await expect(link).toHaveAttribute('aria-current', 'true');
  await expect(page.locator('.inventory-item')).toHaveCount(0);
  await expect(main.getByRole('table')).toHaveCount(1);
  await expect(dialog.getByRole('heading', { name: 'media-services', exact: true })).toBeVisible();
  await dialog.getByRole('tab', { name: 'Configuration' }).click();
  await expect(dialog.getByRole('tabpanel')).toContainText('/projects/media-services');
  await page.keyboard.press('ArrowLeft');
  await expect(dialog.getByRole('tab', { name: 'Overview' })).toBeFocused();
  await expect(dialog.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
  if (testInfo.project.name === 'chromium') {
    await dialog.locator('.resource-details-header').click({ position: { x: 8, y: 8 } });
    for (const theme of ['dark', 'light']) {
      await page.emulateMedia({ colorScheme: theme });
      await page.screenshot({ path: `test-results/resource-details-stack-${theme}.png` });
    }
  }
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(link).toBeFocused();
  await expect(link).not.toHaveAttribute('aria-current');
  await main.getByRole('cell', { name: 'Active', exact: true }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole('link', { name: 'Close details' }).click();
  expect(cspErrors).toEqual([]);
});

test('container selection replaces pending requests, reports errors and cannot reopen after closing', async ({ page }, testInfo) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.setViewportSize({ width: 1440, height: 900 });
  let release;
  let fulfilled;
  let hold = true;
  let fail = false;
  await page.route('**/containers*', async route => {
    const selected = new URL(route.request().url()).searchParams.get('details');
    if (selected === firstID && hold) {
      await new Promise(resolve => { release = resolve; });
      await serve(route);
      fulfilled();
      return;
    }
    if (selected && fail) return route.fulfill({ status: 503, body: 'Snapshot unavailable' });
    return serve(route);
  });
  await page.goto('/containers');
  const first = page.getByRole('link', { name: 'media-services-web-1', exact: true });
  const second = page.getByRole('link', { name: 'media-services-database-1', exact: true });
  await first.click();
  await expect(page.locator('[data-resource-details-request]')).toContainText('Loading details');
  await expect.poll(() => typeof release).toBe('function');
  const aborted = page.waitForEvent('requestfailed', request => request.url().includes(firstID));
  await second.click();
  await aborted;
  const dialog = page.getByRole('dialog');
  await expect(dialog).toHaveAccessibleName('media-services-database-1');
  const late = new Promise(resolve => { fulfilled = resolve; });
  release();
  await late;
  await expect(dialog).toHaveAccessibleName('media-services-database-1');
  hold = false;
  fail = true;
  await first.click();
  await expect(dialog.getByRole('alert')).toContainText('Details could not be loaded');
  await expect(dialog).toHaveAccessibleName('media-services-database-1');
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  fail = false;
  hold = true;
  release = null;
  await first.click();
  await expect(page.locator('[data-resource-details-request]')).toContainText('Loading details');
  await expect.poll(() => typeof release).toBe('function');
  const cancelled = page.waitForEvent('requestfailed', request => request.url().includes(firstID));
  await page.getByRole('button', { name: 'Dismiss' }).click();
  await cancelled;
  const closed = new Promise(resolve => { fulfilled = resolve; });
  release();
  await closed;
  hold = false;
  await second.click();
  await expect(dialog).toHaveAccessibleName('media-services-database-1');
  await page.keyboard.press('Escape');
  await first.click();
  await expect(dialog).toHaveAccessibleName('media-services-web-1');
  await expect(dialog.getByRole('alert')).toBeHidden();
  if (testInfo.project.name === 'chromium') {
    for (const theme of ['dark', 'light']) {
      await page.emulateMedia({ colorScheme: theme });
      await page.screenshot({ path: `test-results/resource-details-container-${theme}.png` });
    }
    await dialog.getByRole('tab', { name: 'Configuration' }).click();
    await dialog.locator('.resource-details-header').click({ position: { x: 8, y: 8 } });
    await page.screenshot({ path: 'test-results/resource-details-container-configuration.png' });
  }
  expect(errors).toEqual([]);
});

test('narrow details use the full screen, contain long content and scroll independently', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 720 });
  await page.route('**/containers*', route => {
    return serve(route, new URL(route.request().url()).searchParams.has('details') ? 'container-long' : 'containers');
  });
  await page.goto('/containers');
  const link = page.getByRole('link', { name: 'media-services-web-1', exact: true });
  if (testInfo.project.use.hasTouch) await link.tap();
  else await link.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  expect(await dialog.evaluate(element => element.matches(':modal'))).toBe(true);
  const bounds = await dialog.boundingBox();
  expect(bounds.x).toBe(0);
  expect(bounds.y).toBe(0);
  expect(bounds.width).toBe(390);
  expect(bounds.height).toBe(720);
  await dialog.getByRole('tab', { name: 'Configuration' }).click();
  const body = dialog.locator('.resource-details-body');
  expect(await body.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
  const scrollBefore = await page.evaluate(() => document.scrollingElement.scrollTop);
  await body.evaluate(element => { element.scrollTop = element.scrollHeight; });
  expect(await body.evaluate(element => element.scrollTop)).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.scrollingElement.scrollTop)).toBe(scrollBefore);
  await page.keyboard.press('Tab');
  expect(await page.evaluate(() => Boolean(document.activeElement.closest('dialog')))).toBe(true);
  if (testInfo.project.name === 'mobile-chromium') {
    await page.screenshot({ path: 'test-results/resource-details-container-mobile-overflow.png' });
  }
  await dialog.getByRole('link', { name: 'Close details' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(link).toBeFocused();
});

test.describe('without JavaScript', () => {
  test.use({ javaScriptEnabled: false });
  test('selected details expose both sections and close through an ordinary inventory link', async ({ page }) => {
    await routeInventory(page);
    await page.goto(`/containers?details=${firstID}`);
    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('Compose association');
    await expect(dialog).toContainText('Container ID');
    await dialog.getByRole('link', { name: 'Close details' }).click();
    await expect(page).toHaveURL(/\/containers$/);
    await expect(page.getByRole('dialog')).toHaveCount(0);
  });
});
