import { test, expect } from '@playwright/test';

test('health endpoint returns JSON liveness and security headers', async ({ request }) => {
  const response = await request.get('/healthz');

  expect(response.status()).toBe(200);
  expect(response.headers()['content-type']).toBe('application/json');
  expect(await response.json()).toEqual({ status: 'ok' });
  expect(response.headers()['x-content-type-options']).toBe('nosniff');
  expect(response.headers()['referrer-policy']).toBe('same-origin');
  expect(response.headers()['content-security-policy']).toBe(
    "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
  );
});

test('HEAD preserves health metadata without a body', async ({ request }) => {
  const response = await request.head('/healthz');

  expect(response.status()).toBe(200);
  expect(response.headers()['content-type']).toBe('application/json');
  expect(await response.body()).toHaveLength(0);
});

test('unknown paths are not served the application shell', async ({ request }) => {
  const response = await request.get('/missing');

  expect(response.status()).toBe(404);
  expect(await response.text()).toContain('404 page not found');
});

test('health endpoint rejects mutations', async ({ request }) => {
  const response = await request.post('/healthz');

  expect(response.status()).toBe(405);
  expect(response.headers().allow).toBe('GET, HEAD');
});

test('application home redirects unauthenticated requests to login', async ({ request }) => {
  const response = await request.get('/', { maxRedirects: 0 });

  expect(response.status()).toBe(303);
  expect(response.headers().location).toBe('/login');
});

test('login page escapes input-free copy and includes a CSRF form token', async ({ request }) => {
  const response = await request.get('/login');

  expect(response.status()).toBe(200);
  const html = await response.text();
  expect(html).toContain('Welcome back');
  expect(html).toContain('name="csrf_token"');
  expect(html).toContain('autocomplete="current-password"');
  expect(html).not.toContain('Hello,');
});

test('htmx is served locally', async ({ request }) => {
  const response = await request.get('/static/htmx.min.js');

  expect(response.status()).toBe(200);
  expect(response.headers()['content-type']).toContain('javascript');
  expect(await response.text()).toContain('htmx');
});