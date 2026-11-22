import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

const port = process.env.PLAYWRIGHT_PORT || '18080';
const baseURL = `http://127.0.0.1:${port}`;
const testDataDir = mkdtempSync(path.join(tmpdir(), 'compose-manager-e2e-'));
process.on('exit', () => rmSync(testDataDir, { recursive: true, force: true }));

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  workers: 2,
  timeout: 30_000,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'go run ./cmd/server',
    url: `${baseURL}/healthz`,
    env: {
      APP_ADDR: `127.0.0.1:${port}`,
      APP_DATA_DIR: testDataDir,
      APP_PUBLIC_URL: baseURL,
      APP_BOOTSTRAP_USERNAME: 'playwright-admin',
      APP_BOOTSTRAP_PASSWORD: 'playwright-temporary-bootstrap-password',
    },
    reuseExistingServer: false,
    timeout: 60_000,
  },
  projects: [
    { name: 'js', testMatch: 'js/**/*.spec.js' },
    {
      name: 'chromium',
      testMatch: 'e2e/**/*.spec.js',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      testMatch: 'e2e/**/*.spec.js',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      testMatch: 'e2e/**/*.spec.js',
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'mobile-chromium',
      testMatch: 'e2e/**/*.spec.js',
      use: { ...devices['Pixel 7'] },
    },
  ],
});