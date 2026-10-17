import assert from 'node:assert/strict';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { after, before, test, type TestContext } from 'node:test';
import { chromium, expect, type Browser, type BrowserContextOptions, type Page } from '@playwright/test';

const baseURL = 'http://127.0.0.1:3847';
let server: ChildProcess;
let browser: Browser;

before(async () => {
  server = spawn('/usr/local/bin/switchboard', ['--listen-host', '127.0.0.1'], { stdio: 'inherit' });
  await once(server, 'spawn');
  await expect.poll(async () => {
    const response = await fetch(baseURL).catch(() => null);
    const status = response?.status;
    return status;
  }, { timeout: 15000 }).toBe(200);
  browser = await chromium.launch();
});

after(async () => {
  await browser?.close();
  const running = server?.pid && server.exitCode === null;
  if (!running) return;
  server.kill('SIGTERM');
  await once(server, 'exit');
});

[1280, 390].forEach(width => {
  test(`existing navigation remains usable at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/integrations');
    await expect(page.locator('.nav-item')).toHaveCount(5);
    await page.locator('.nav-item[href="/settings"]').click();
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
    await expect(page.locator('.card').first()).toHaveCSS('padding', '16px');
    await page.locator('.nav-item[href="/integrations"]').click();
    await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible();
  });
});

(['light', 'dark'] as const).forEach(colorScheme => {
  test(`system ${colorScheme} styles work without JavaScript`, async (t) => {
    const page = await browserPage(t, { javaScriptEnabled: false, colorScheme });
    await page.goto('/settings');
    await expect(page.locator('html')).toHaveCSS('color-scheme', colorScheme);
    await expect(page.locator('select[name="session_store"]')).toHaveCSS('appearance', 'none');
    await expect(page.getByRole('button', { name: 'Save Settings' })).toBeEnabled();
  });
});

test('shared buttons use neutral colors and accent hover', async (t) => {
  const page = await browserPage(t);
  await page.goto('/settings');
  await page.addStyleTag({ content: ':root { --surface: #123456; --text: #fedcba; --border: #abcdef; --accent: #3399ff; }' });
  const button = page.getByRole('button', { name: 'Save Settings' });
  await expect(button).toHaveCSS('background-color', 'rgb(18, 52, 86)');
  await expect(button).toHaveCSS('color', 'rgb(254, 220, 186)');
  await expect(button).toHaveCSS('border-color', 'rgb(171, 205, 239)');
  await button.hover();
  await expect(button).toHaveCSS('color', 'rgb(51, 153, 255)');
  await expect(button).toHaveCSS('border-color', 'rgb(51, 153, 255)');
});

test('styled settings form still saves through a native submission', async (t) => {
  const page = await browserPage(t);
  await page.goto('/settings');
  const rate = page.locator('input[name="dollars_per_mtok_input"]');
  await rate.fill('4.25');
  await page.getByRole('button', { name: 'Save Settings' }).click();
  await page.waitForLoadState();
  await page.reload();
  await expect(rate).toHaveValue('4.25');
});

test('custom select chevrons retain native keyboard selection', async (t) => {
  const page = await browserPage(t);
  await page.goto('/settings');
  const select = page.locator('select[name="session_store"]');
  await expect(select).toHaveCSS('background-image', /^url\("data:image\/svg\+xml,/);
  await select.focus();
  await page.keyboard.press('ArrowDown');
  await expect(select).toHaveValue('file');
  await expect(select).toBeFocused();
});

test('select chevrons restore native appearance in forced colors', async (t) => {
  const page = await browserPage(t, { forcedColors: 'active' });
  await page.goto('/settings');
  const select = page.locator('select[name="session_store"]');
  await expect(select).toHaveCSS('appearance', 'auto');
  await expect(select).toHaveCSS('background-image', 'none');
});

[
  { status: 'success', cssVariable: '--green' },
  { status: 'error', cssVariable: '--red' },
].forEach(({ status, cssVariable }) => {
  test(`${status} banners share their text and border color`, async (t) => {
    const page = await browserPage(t);
    await page.goto(`/settings?${status}=Example`);
    const content = `:root { ${cssVariable}: #3399ff; }`;
    await page.addStyleTag({ content });
    const message = page.locator(`.flash-${status}`);
    await expect(message).toHaveText('Example');
    await expect(message).toHaveCSS('color', 'rgb(51, 153, 255)');
    await expect(message).toHaveCSS('border-color', 'rgb(51, 153, 255)');
  });
});

test('reduced motion disables shared control transitions', async (t) => {
  const page = await browserPage(t, { reducedMotion: 'reduce' });
  await page.goto('/settings');
  await expect(page.getByRole('button', { name: 'Save Settings' })).toHaveCSS('transition-duration', '0s');
});

test('stylesheet is served without loading a runtime CSS compiler', async (t) => {
  const page = await browserPage(t);
  const response = page.waitForResponse(`${baseURL}/static/style.css`);
  await page.goto('/settings');
  assert.equal((await response).status(), 200);
  await expect(page.locator('script[src*="unocss"], script[src="/static/style.js"]')).toHaveCount(0);
});

async function browserPage(t: TestContext, options: BrowserContextOptions = {}): Promise<Page> {
  const context = await browser.newContext({ baseURL, ...options });
  t.after(() => context.close());
  const page = await context.newPage();
  return page;
}
