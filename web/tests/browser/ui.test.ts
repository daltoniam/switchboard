import assert from 'node:assert/strict';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { after, before, test, type TestContext } from 'node:test';
import { chromium, expect, type Browser, type BrowserContextOptions, type Locator, type Page } from '@playwright/test';

const baseURL = 'http://127.0.0.1:3847';
const scriptFixtures = [
  ['https://cdn.jsdelivr.net/npm/htmx.org@2.0.11/dist/htmx.min.js', '/tests/node_modules/htmx.org/dist/htmx.min.js'],
  ['https://cdn.jsdelivr.net/npm/@tarekraafat/autocomplete.js@10.2.10/dist/autoComplete.min.js', '/tests/node_modules/@tarekraafat/autocomplete.js/dist/autoComplete.min.js'],
];
let server: ChildProcess;
let browser: Browser;
let logoSources: string[];

before(async () => {
  server = spawn('/usr/local/bin/switchboard', ['--listen-host', '127.0.0.1'], { stdio: 'inherit' });
  await once(server, 'spawn');
  await expect.poll(async () => {
    const response = await fetch(baseURL).catch(() => null);
    const status = response?.status;
    return status;
  }, { timeout: 15000 }).toBe(200);
  const response = await fetch(`${baseURL}/static/style.css`);
  const css = await response.text();
  logoSources = [...new Set(css.match(/https:\/\/cdn\.jsdelivr\.net\/gh\/[\w-]+\/[\w-]+@[a-f0-9]{40}\/[\w/.-]+\.svg/g))];
  assert.ok(logoSources.length > 0);
  browser = await chromium.launch();
});

after(async () => {
  await browser?.close();
  const running = server?.pid && server.exitCode === null;
  if (!running) return;
  server.kill('SIGTERM');
  await once(server, 'exit');
});

test('navigation keeps the sidebar and icons styled', async (t) => {
  const page = await browserPage(t);
  await page.goto('/integrations');
  await expect(page.locator('.sidebar')).toHaveCSS('width', '200px');
  const icons = page.locator('.nav-item > span');
  await expect(icons).toHaveCount(5);
  await assertIcons(icons);
  await page.getByRole('link', { name: 'Settings', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '200px');
  await page.getByRole('link', { name: 'Integrations', exact: true }).click();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '200px');
});

test('pinned CDN scripts support navigation search with isolated fixtures', async (t) => {
  const page = await browserPage(t);
  await page.goto('/integrations');
  const sources = await page.locator('script[src]').evaluateAll(scripts => scripts.map(script => script.src));
  const expected = scriptFixtures.map(([url]) => url);
  assert.deepEqual(sources, [...expected, `${baseURL}/static/ui.mjs`]);
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  const search = page.getByRole('combobox', { name: 'Search pages, integrations, and projects' });
  await expect(search).toBeEnabled();
  await search.fill('Settings');
  await page.getByRole('option', { name: 'Settings', exact: true }).click();
  await expect(page).toHaveURL(`${baseURL}/settings`);
});

test('navigation search preserves server matches outside project titles', async (t) => {
  const page = await browserPage(t);
  const json = [{ label: 'Project: Website', path: '/projects/website' }];
  await page.route('**/api/navigation?q=payments', route => route.fulfill({ json }));
  await page.goto('/integrations');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.getByRole('combobox', { name: 'Search pages, integrations, and projects' }).fill('payments');
  const results = page.getByRole('option');
  await expect(results).toHaveCount(1);
  await expect(results).toHaveText(['Project: Website']);
  await expect(page.locator('#search-status')).toHaveText('1 results. Use arrow keys to choose.');
  await results.click();
  await expect(page).toHaveURL(`${baseURL}/projects/website`);
});

[200, 503].forEach(status => {
  test(`navigation search filters page links when the API returns ${status}`, async (t) => {
    const page = await browserPage(t);
    const json = [];
    await page.route('**/api/navigation?*', route => route.fulfill({ status, json }));
    await page.goto('/integrations');
    await page.getByRole('button', { name: 'Search', exact: true }).click();
    const search = page.getByRole('combobox', { name: 'Search pages, integrations, and projects' });
    await search.fill('sEtTiNgS');
    await expect(page.getByRole('option')).toHaveText(['Settings']);
    await search.fill('no-matching-page');
    await expect(page.getByRole('option')).toHaveCount(0);
  });
});

test('branded logo variants load from pinned CDN URLs with isolated image fixtures', async (t) => {
  const page = await browserPage(t);
  const loaded = page.waitForResponse(response => response.url().endsWith('/github_light.svg') && response.ok());
  await page.goto('/integrations');
  await page.evaluate(() => { document.documentElement.dataset.theme = 'github-light'; });
  const logo = page.locator('.integration-icon[data-integration="github"]').first();
  await expect(logo).toHaveCSS('background-image', /\/svgl@[a-f0-9]{40}\/static\/library\/github_light\.svg/);
  await loaded;
  const darkLoaded = page.waitForResponse(response => response.url().endsWith('/github_dark.svg') && response.ok());
  await page.evaluate(() => { document.documentElement.dataset.theme = 'github-dark'; });
  await expect(logo).toHaveCSS('background-image', /\/svgl@[a-f0-9]{40}\/static\/library\/github_dark\.svg/);
  await darkLoaded;
});

test('project filtering targets results and preserves the total count', async (t) => {
  const page = await browserPage(t);
  await page.goto('/projects');
  const count = page.getByText('0 projects', { exact: true });
  await expect(count).toHaveText('0 projects');
  const filtered = page.waitForResponse(response => response.url().includes('/projects?q=alpha'));
  await page.getByRole('searchbox', { name: 'Filter projects' }).fill('alpha');
  const response = await filtered;
  assert.equal(response.status(), 200);
  assert.equal(response.request().headers()['hx-target'], 'project-results');
  const body = await response.text();
  assert.match(body, /^<div id="project-results"/);
  assert.doesNotMatch(body, /<html|id="project-search"/);
  await expect(page).toHaveURL(`${baseURL}/projects?q=alpha`);
  await expect(page.locator('#project-results')).toContainText('No projects match this view.');
  await expect(page.getByRole('searchbox', { name: 'Filter projects' })).toBeFocused();
  await expect(count).toHaveText('0 projects');
});

test('failed project filtering preserves results and the search input', async (t) => {
  const page = await browserPage(t);
  await page.goto('/projects');
  const results = await page.locator('#project-results').innerHTML();
  await page.route('**/projects?q=alpha', route => route.fulfill({ status: 500 }));
  const input = page.getByRole('searchbox', { name: 'Filter projects' });
  await input.fill('alpha');
  await expect(page.getByRole('alert')).toHaveText('Could not filter projects. Try again.');
  await expect(input).toHaveValue('alpha');
  await expect(input).toBeFocused();
  assert.equal(await page.locator('#project-results').innerHTML(), results);
  await expect(page).toHaveURL(`${baseURL}/projects`);
});

test('project filtering cancels an older page refresh without a failure toast', async (t) => {
  const page = await browserPage(t);
  await page.goto('/projects');
  const refresh = page.getByRole('button', { name: 'Refresh projects' });
  const held = await holdProjectRequest(page, '');
  await refresh.click();
  await held.started;
  await expect(refresh).toBeDisabled();
  await expect(refresh).toHaveAttribute('aria-busy', 'true');
  await expect(page.locator('main')).toHaveClass(/htmx-request/);
  const aborted = page.waitForEvent('requestfailed', { predicate: request => request.url() === `${baseURL}/projects`, timeout: 5000 });
  await page.getByRole('searchbox', { name: 'Filter projects' }).fill('alpha');
  await aborted;
  await held.release();
  await expect(page).toHaveURL(`${baseURL}/projects?q=alpha`);
  await expect(refresh).toBeEnabled();
  await expect(refresh).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('main')).not.toHaveClass(/htmx-request/);
  await expect(page.locator('#page-refresh-status')).toBeHidden();
  await expect(page.locator('.section-loading')).toHaveCount(0);
});

test('project refresh unlocks while its success message remains visible', async (t) => {
  const page = await browserPage(t);
  await page.goto('/projects?q=alpha');
  const button = page.getByRole('button', { name: 'Refresh projects' });
  assert.equal(await button.getAttribute('data-page-refresh'), null);
  await button.click();
  await expect(page.locator('#page-refresh-status')).toHaveText('Refreshed.');
  await expect(button).toBeEnabled();
  await expect(page.locator('#page-refresh-status')).toBeVisible();
  await expect(page.getByRole('searchbox', { name: 'Filter projects' })).toHaveValue('alpha');
});

test('project refresh does not interrupt a pending filter', async (t) => {
  const page = await browserPage(t);
  await page.goto('/projects');
  const held = await holdProjectRequest(page, '?q=alpha', 'project-results');
  await page.getByRole('searchbox', { name: 'Filter projects' }).fill('alpha');
  await held.started;
  const requests = new Set<string>();
  page.on('request', request => { requests.add(request.url()); });
  await page.getByRole('button', { name: 'Refresh projects' }).click();
  await held.release();
  await expect(page).toHaveURL(`${baseURL}/projects?q=alpha`);
  assert.equal(requests.has(`${baseURL}/projects`), false);
  await expect(page.getByRole('searchbox', { name: 'Filter projects' })).toHaveValue('alpha');
  await expect(page.locator('#page-refresh-status')).toBeHidden();
});

async function holdProjectRequest(page: Page, query: string, target = 'main') {
  const headers = { 'HX-Request': 'true', 'HX-Target': target };
  const response = await page.request.get(`/projects${query}`, { headers });
  const body = await response.text();
  const { promise: started, resolve: markStarted } = Promise.withResolvers<void>();
  const release = Promise.withResolvers<void>();
  const completed = Promise.withResolvers<void>();
  await page.route(`${baseURL}/projects${query}`, async route => {
    markStarted();
    await release.promise;
    await route.fulfill({ body, contentType: 'text/html' });
    completed.resolve();
  }, { times: 1 });
  const finish = async () => { release.resolve(); await completed.promise; };
  const held = { started, release: finish };
  return held;
}

[1280, 768, 600].forEach(width => {
  test(`sidebar preserves label width and padding at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/styleguide');
    for (const size of ['80', '100', '120']) {
      await selectFontSize(page, size);
      await openSidebar(page);
      await assertSidebarLabels(page);
    }
    await page.locator('.nav-item[href="/plugins"]').evaluate(link => {
      link.lastChild.textContent = 'Installed Plugin Marketplace';
    });
    await assertSidebarLabels(page);
  });
});

test('dismissing a flash preserves the URL and survives reload', async (t) => {
  const page = await browserPage(t);
  await page.goto('/integrations?success=Saved&filter=enabled#connected');
  await page.getByRole('button', { name: 'Dismiss success message' }).click();
  await expect(page.locator('[data-flash="success"]')).toHaveCount(0);
  await expect(page.locator('main')).toBeFocused();
  assert.equal(page.url(), `${baseURL}/integrations?filter=enabled#connected`);
  await page.reload();
  await expect(page.locator('[data-flash="success"]')).toHaveCount(0);
});

test('health refresh replaces results without navigating', async (t) => {
  const page = await browserPage(t);
  await page.goto('/integrations');
  const button = page.getByRole('button', { name: 'Re-check all' });
  const response = page.waitForResponse('**/api/health/refresh');
  await button.click();
  assert.equal((await response).request().headers()['hx-request'], 'true');
  await expect(page.getByRole('status')).toHaveText('Health checks complete.');
  await expect(page.locator('#integration-results')).toHaveCount(1);
  await expect(page.locator('.integration-card').first()).toHaveCSS('display', 'flex');
  await expect(button).toBeEnabled();
  await expect(page.getByRole('status')).toHaveText('Health checks complete.');
});

test('failed health refresh allows retry while the error remains visible', async (t) => {
  const page = await browserPage(t);
  const pending = Promise.withResolvers<void>();
  await page.route('**/api/health/refresh', async route => {
    await pending.promise;
    await route.fulfill({ status: 500 });
  }, { times: 1 });
  await page.goto('/integrations');
  const button = page.getByRole('button', { name: 'Re-check all' });
  await button.click();
  await expect(button).toBeDisabled();
  await expect(page.locator('#health-refresh')).toHaveAttribute('aria-busy', 'true');
  pending.resolve();
  await expect(page.getByRole('alert')).toHaveText('Could not refresh. Try again.');
  await expect(button).toBeEnabled();
  await expect(page.getByRole('alert')).toHaveText('Could not refresh. Try again.');
  await button.click();
  await expect(page.getByRole('status')).toHaveText('Health checks complete.');
});

test('network failure clears loading state and permits refresh retry', async (t) => {
  const page = await browserPage(t);
  await page.route('**/api/health/refresh', route => route.abort('failed'), { times: 1 });
  await page.goto('/integrations');
  const button = page.getByRole('button', { name: 'Re-check all' });
  await button.click();
  await expect(page.getByRole('alert')).toHaveText('Could not refresh. Try again.');
  await expect(button).toBeEnabled();
  await expect(page.locator('#health-refresh')).toHaveAttribute('aria-busy', 'false');
  await expect(page.locator('.section-loading')).toHaveCount(0);
  await button.click();
  await expect(page.getByRole('status')).toHaveText('Health checks complete.');
});

[390, 768, 1024].forEach(width => {
  test(`mobile navigation and styles work without JavaScript at ${width}px`, async (t) => {
    const viewport = { width, height: 844 };
    const page = await browserPage(t, { javaScriptEnabled: false, viewport });
    await page.goto('/integrations');
    const sidebar = await page.locator('.sidebar').boundingBox();
    const main = await page.locator('main').boundingBox();
    assert.ok(sidebar.y + sidebar.height <= main.y, 'navigation must sit above the page without JavaScript');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    assert.equal(overflow, false);
    await assertIcons(page.locator('.nav-item > span'));
    await page.getByRole('button', { name: 'Re-check all' }).click();
    await expect(page).toHaveURL(`${baseURL}/integrations`);
    await expect(page.locator('#integration-results')).toBeVisible();
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
  });
});

['**/static/ui.mjs', '**/htmx.min.js'].forEach(script => {
  test(`navigation remains usable when ${script} cannot initialize`, async (t) => {
    const viewport = { width: 390, height: 844 };
    const page = await browserPage(t, { viewport });
    await page.addInitScript(() => localStorage.setItem('switchboard.sidebar', 'closed'));
    await page.route(script, route => route.abort());
    await page.goto('/integrations');
    await expect(page.getByRole('button', { name: 'Toggle sidebar' })).toBeDisabled();
    await expect(page.locator('.sidebar')).toBeVisible();
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Settings', exact: true })).toBeVisible();
  });
});

test('blocked storage preserves navigation and appearance controls', async (t) => {
  const viewport = { width: 390, height: 844 };
  const page = await browserPage(t, { viewport });
  const errors = new Set<string>();
  page.on('pageerror', error => { errors.add(error.message); });
  await page.addInitScript(() => {
    Object.defineProperty(window, 'localStorage', {
      get() { throw new DOMException('Storage denied', 'SecurityError'); },
    });
  });
  await page.goto('/integrations');
  await expect(page.getByRole('button', { name: 'Toggle sidebar' })).toBeEnabled();
  await page.getByRole('button', { name: 'Toggle sidebar' }).click();
  await expect(page.locator('.sidebar')).toBeVisible();
  await selectFontSize(page, '120');
  await expect(page.locator('html')).toHaveAttribute('data-font-size', '120');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('combobox')).toBeEnabled();
  assert.equal(errors.size, 0);
});

test('mobile sidebar still opens and closes with JavaScript', async (t) => {
  const viewport = { width: 390, height: 844 };
  const page = await browserPage(t, { viewport });
  await page.goto('/integrations');
  const sidebar = page.locator('.sidebar');
  const toggle = page.getByRole('button', { name: 'Toggle sidebar' });
  await expect(sidebar).toBeHidden();
  await toggle.click();
  await expect(sidebar).toBeVisible();
  await expect(sidebar).toHaveCSS('position', 'fixed');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await page.reload();
  await expect(sidebar).toBeVisible();
  await expect(toggle).toBeEnabled();
  await page.getByRole('button', { name: 'Close sidebar' }).click();
  await expect(sidebar).toBeHidden();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(toggle).toBeFocused();
});

[1280, 390].forEach(width => {
  test(`shared text remains readable at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/integrations?success=Saved');
    await expect(page.locator('body')).toHaveCSS('font-size', '16px');
    const text = page.locator('.nav-item, .btn:not(.font-size-option), .section-title, .integration-card-tools, .footer');
    const sizes = await text.evaluateAll(elements => elements.map(element => {
      const { fontSize } = getComputedStyle(element);
      const size = Number.parseFloat(fontSize);
      return size;
    }));
    assert.ok(sizes.length > 0);
    assert.ok(sizes.every(size => size >= 14), JSON.stringify(sizes));
    for (const badge of await page.locator('.badge').all()) {
      await expect(badge).toHaveCSS('font-size', '12px');
      await expect(badge).toHaveCSS('border-style', 'solid');
      await expect(badge).toHaveCSS('border-width', '1px');
    }
  });
});

['.toggle-group-item:not(:disabled)', '.nav-item[href="/settings"]', '.integration-card'].forEach(selector => {
  test(`${selector} shares blue hover and keyboard focus`, async (t) => {
    const page = await browserPage(t);
    await page.goto('/integrations');
    await page.addStyleTag({ content: ':root { --accent: #3399ff; --accent-dim: #123456; }' });
    const control = page.locator(selector).first();
    await control.hover();
    await expect(control).toHaveCSS('color', 'rgb(51, 153, 255)');
    await expect(control).toHaveCSS('background-color', 'rgb(18, 52, 86)');
    await page.mouse.move(0, 0);
    await page.keyboard.press('Tab');
    await control.focus();
    await expect(control).toHaveCSS('outline-color', 'rgb(51, 153, 255)');
    await expect(control).toHaveCSS('outline-style', 'solid');
    await expect(control).toHaveCSS('outline-width', '2px');
  });
});

test('style guide stays out of navigation and disabled buttons stay neutral', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  await expect(page.getByRole('heading', { name: 'Style guide', exact: true })).toBeVisible();
  await expect(page.locator('nav a[href="/styleguide"]')).toHaveCount(0);
  const disabled = page.getByRole('button', { name: 'Disabled action' });
  await page.addStyleTag({ content: ':root { --surface: #123456; --text: #fedcba; }' });
  await disabled.hover({ force: true });
  await expect(disabled).toBeDisabled();
  await expect(disabled).toHaveCSS('background-color', 'rgb(18, 52, 86)');
  await expect(disabled).toHaveCSS('color', 'rgb(254, 220, 186)');
});

test('delete buttons keep their red border and text through interaction states', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  await page.addStyleTag({ content: ':root { --red: #ff3344; --red-dim: #331122; --accent: #3399ff; }' });
  const buttons = page.locator('.identity-delete');
  await expect(buttons).toHaveCount(2);
  for (const button of await buttons.all()) {
    await expect(button).toHaveCSS('color', 'rgb(255, 51, 68)');
    await expect(button).toHaveCSS('border-color', 'rgb(255, 51, 68)');
    await button.hover();
    await expect(button).toHaveCSS('color', 'rgb(255, 51, 68)');
    await expect(button).toHaveCSS('border-color', 'rgb(255, 51, 68)');
    await expect(button).toHaveCSS('background-color', 'rgb(51, 17, 34)');
    await page.mouse.down();
    await expect(button).toHaveCSS('border-color', 'rgb(255, 51, 68)');
    await page.mouse.up();
    await page.mouse.move(0, 0);
    await page.keyboard.press('Tab');
    await button.focus();
    await expect(button).toHaveCSS('outline-color', 'rgb(255, 51, 68)');
  }
});

[
  { status: 'success', cssVariable: '--green' },
  { status: 'error', cssVariable: '--red' },
].forEach(({ status, cssVariable }) => {
  test(`${status} shares its theme color across text, border, and background`, async (t) => {
    const page = await browserPage(t);
    await page.goto('/styleguide');
    const message = page.locator(`.flash-${status}`);
    const background = await message.evaluate(element => {
      const { backgroundColor } = getComputedStyle(element);
      return backgroundColor;
    });
    const content = `:root { ${cssVariable}: #3399ff; }`;
    await page.addStyleTag({ content });
    await expect(message).toHaveCSS('color', 'rgb(51, 153, 255)');
    await expect(message).toHaveCSS('border-color', 'rgb(51, 153, 255)');
    await expect(message).toHaveCSS('border-style', 'solid');
    await expect(message).toHaveCSS('border-width', '1px');
    await expect(message).not.toHaveCSS('background-color', background);
  });
});

test('style guide form examples are interactive without saving', async (t) => {
  const page = await browserPage(t);
  const mutations = new Set<string>();
  page.on('request', request => {
    const isMutation = !['GET', 'HEAD'].includes(request.method());
    if (isMutation) mutations.add(request.url());
  });
  await page.goto('/styleguide');
  await page.getByRole('button', { name: 'Toggle visibility' }).click();
  await expect(page.getByLabel('Secret input')).toHaveAttribute('type', 'text');
  await page.getByLabel('Select', { exact: true }).selectOption({ label: 'File' });
  await page.getByLabel('Personal', { exact: true }).check();
  const setting = page.getByLabel('Example setting', { exact: true });
  await page.locator('.toggle').filter({ has: setting }).click();
  await expect(setting).toBeChecked();
  await page.getByLabel('Multiline input').fill('Example configuration');
  await page.getByRole('button', { name: 'Save example', exact: true }).click();
  assert.equal(mutations.size, 0);
});

['success', 'error'].forEach(status => {
  test(`${status} dismisses with a themed Lucide close icon`, async (t) => {
    const page = await browserPage(t);
    await page.goto('/styleguide?filter=enabled#theme-statuses');
    const banner = page.locator(`.flash-${status}`);
    const name = `Dismiss example ${status}`;
    const button = banner.getByRole('button', { name });
    await expect(button).toHaveText('');
    await expect(button).toHaveCSS('appearance', 'none');
    await expect(button).toHaveCSS('border-style', 'none');
    await expect(button).toHaveCSS('box-shadow', /^(none|(?:rgba\(0, 0, 0, 0\) 0px 0px 0px 0px(?:, )?)+)$/);
    await expect(button.locator('[aria-hidden="true"]')).toHaveCSS('mask-image', /data:image\/svg\+xml/);
    const color = await banner.evaluate(element => getComputedStyle(element).color);
    await page.addStyleTag({ content: ':root { --accent-dim: #123456; }' });
    await button.hover();
    await expect(button).toHaveCSS('color', color);
    await expect(button).toHaveCSS('border-width', '0px');
    await expect(button).toHaveCSS('background-color', 'rgb(18, 52, 86)');
    await button.focus();
    await button.press('Enter');
    await expect(banner).toHaveCount(0);
    assert.equal(page.url(), `${baseURL}/styleguide?filter=enabled#theme-statuses`);
  });
});

test('error cards share the error message palette while zero errors stay neutral', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  const palette = await page.locator('.flash-error').evaluate(element => {
    const { color, borderColor, backgroundColor } = getComputedStyle(element);
    const colors = { color, borderColor, backgroundColor };
    return colors;
  });
  const cards = page.locator('.stat-card, .metric-card, .efficiency-item').filter({ hasText: /^(2Errors|1%Error rate|1Truncations)$/ });
  await expect(cards).toHaveCount(3);
  for (const card of await cards.all()) {
    await expect(card).toHaveCSS('color', palette.color);
    await expect(card).toHaveCSS('border-color', palette.borderColor);
    await expect(card).toHaveCSS('background-color', palette.backgroundColor);
  }
  const neutral = page.locator('.stat-card').filter({ hasText: 'Zero errors' });
  await expect(neutral).not.toHaveCSS('border-color', palette.borderColor);
  await expect(neutral).not.toHaveCSS('background-color', palette.backgroundColor);
});

[1280, 390].forEach(width => {
  test(`style guide sections fit at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/styleguide');
    const headings = ['Typography', 'Form controls', 'Cards and integrations', 'Metrics and summaries',
      'Tables and metadata', 'Lists and tools', 'Identity editor', 'Setup steps and code', 'Empty states'];
    for (const name of headings) await expect(page.getByRole('heading', { name, exact: true })).toBeVisible();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    assert.equal(overflow, false);
    await page.getByRole('button', { name: 'Dismiss example success' }).click();
    await expect(page.locator('[data-flash="success"]')).toHaveCount(0);
  });
});

test('dashboard uptime and healthy counts use neutral colors', async (t) => {
  const page = await browserPage(t);
  await page.goto('/');
  await page.addStyleTag({ content: ':root { --text: #fedcba; --text-muted: #abcdef; --surface-hover: #123456; }' });
  const uptime = page.getByText(/^Uptime:/);
  await expect(uptime).toHaveCSS('color', 'rgb(171, 205, 239)');
  await expect(uptime).toHaveCSS('background-color', 'rgb(18, 52, 86)');
  const connected = page.locator('.stat-card').filter({ hasText: 'Connected' });
  await expect(connected.locator('.stat-value')).toHaveCSS('color', 'rgb(254, 220, 186)');
  const errors = page.locator('.stat-card').filter({ hasText: 'Errored' });
  await expect(errors.locator('.stat-value')).toHaveText('0');
  await expect(errors.locator('.stat-value')).toHaveCSS('color', 'rgb(254, 220, 186)');
});

test('unselected marketplace buttons and file picker share theme colors', async (t) => {
  const page = await browserPage(t);
  await page.goto('/plugins');
  await page.addStyleTag({ content: ':root { --surface: #123456; --text: #fedcba; --border: #abcdef; }' });
  const buttons = page.locator('main button.btn:not([aria-pressed="true"])');
  assert.ok(await buttons.count() > 0);
  const expected = { backgroundColor: 'rgb(18, 52, 86)', color: 'rgb(254, 220, 186)', borderColor: 'rgb(171, 205, 239)' };
  for (const button of await buttons.all()) {
    await expect(button).toHaveCSS('background-color', expected.backgroundColor);
    await expect(button).toHaveCSS('color', expected.color);
    await expect(button).toHaveCSS('border-color', expected.borderColor);
  }
  const fileButton = await page.locator('#plugin_file').evaluate(element => {
    const { backgroundColor, color, borderColor } = getComputedStyle(element, '::file-selector-button');
    const colors = { backgroundColor, color, borderColor };
    return colors;
  });
  assert.deepEqual(fileButton, expected);
});

[
  ['github-light', 'light'], ['atom-one-light', 'light'], ['catppuccin-latte', 'light'],
  ['github-dark', 'dark'], ['dracula', 'dark'], ['nord', 'dark'],
].forEach(([theme, scheme]) => {
  test(`${theme} applies across navigation and reload`, async (t) => {
    const page = await browserPage(t);
    await page.goto('/styleguide');
    await page.locator('[data-theme-picker] > summary').click();
    const themeOption = page.locator(`input[name="theme"][value="${theme}"]`);
    await themeOption.locator('..').click();
    await expect(page.locator('html')).toHaveCSS('color-scheme', scheme);
    await assertSelectChevron(page.getByLabel('Select', { exact: true }));
    await page.getByRole('link', { name: 'Settings', exact: true }).click();
    await page.reload();
    await expect(themeOption).toBeChecked();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await assertSelectChevron(page.locator('select[name="session_store"]'));
  });
});

test('custom select chevrons retain native keyboard selection', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  const select = page.getByLabel('Select', { exact: true });
  await select.focus();
  await page.keyboard.press('ArrowDown');
  await expect(select).toHaveValue('File');
  await expect(select).toBeFocused();
});

test('select chevrons restore native appearance in forced colors', async (t) => {
  const page = await browserPage(t, { forcedColors: 'active' });
  await page.goto('/styleguide');
  const select = page.getByLabel('Select', { exact: true });
  await expect(select).toHaveCSS('appearance', 'auto');
  await expect(select).toHaveCSS('background-image', 'none');
});

[1280, 390].forEach(width => {
  test(`all font sizes reflow without page overflow at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/styleguide');
    for (const size of ['80', '100', '120']) {
      await selectFontSize(page, size);
      const expected = `${16 * Number(size) / 100}px`;
      await expect(page.locator('body')).toHaveCSS('font-size', expected);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      assert.equal(overflow, false, `overflow at ${size}%`);
    }
    await page.reload();
    await expect(page.locator('input[name="font-size"][value="120"]')).toBeChecked();
    for (const path of ['/', '/integrations', '/plugins', '/settings', '/projects']) {
      await page.goto(path);
      await expect(page.locator('input[name="font-size"][value="120"]')).toBeChecked();
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
      assert.equal(overflow, false, `${path} overflows at 120%`);
    }
  });
});

test('saved appearance applies before the deferred UI module loads', async (t) => {
  const page = await browserPage(t);
  await page.addInitScript(() => {
    localStorage.setItem('switchboard.theme', 'nord');
    localStorage.setItem('switchboard.fontSize', '120');
  });
  await page.route('**/static/ui.mjs', route => route.abort());
  await page.goto('/styleguide');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'nord');
  await expect(page.locator('body')).toHaveCSS('font-size', '19.2px');
});

test('system appearance works with JavaScript disabled', async (t) => {
  const page = await browserPage(t, { javaScriptEnabled: false, colorScheme: 'dark' });
  await page.goto('/styleguide');
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
  const themeOptions = page.locator('input[name="theme"]');
  assert.ok(await themeOptions.count() > 0);
  for (const option of await themeOptions.all()) await expect(option).toBeDisabled();
  await expect(page.locator('input[name="font-size"][value="100"]')).toBeDisabled();
});

test('font-size menu uses scaled glyphs and supports arrow-key selection', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  await page.locator('[data-font-picker] > summary').click();
  const glyphs = page.locator('.font-size-option');
  await expect(glyphs).toHaveText(['A', 'A', 'A']);
  const sizes = await glyphs.evaluateAll(elements => elements.map(element => {
    const { fontSize } = getComputedStyle(element);
    return fontSize;
  }));
  assert.deepEqual(sizes, ['11.2px', '14px', '16.8px']);
  await page.getByRole('radio', { name: '100%', exact: true }).focus();
  await page.keyboard.press('ArrowRight');
  const selected = page.getByRole('radio', { name: '120%', exact: true });
  await expect(selected).toBeChecked();
  await expect(page.locator('input[name="font-size"]:checked')).toHaveCount(1);
  await expect(page.locator('html')).toHaveAttribute('data-font-size', '120');
  await expect(selected.locator('..')).toHaveCSS('outline-style', 'solid');
});

[1280, 390].forEach(width => {
  test(`appearance controls share sizing at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/styleguide');
    for (const size of ['80', '100', '120']) {
      await selectFontSize(page, size);
      const controls = await page.locator('.appearance-picker > summary').evaluateAll(elements => elements.map(element => {
        const { height, top } = element.getBoundingClientRect();
        const { fontSize, fontFamily, fontWeight } = getComputedStyle(element);
        const control = { height, top, fontSize, fontFamily, fontWeight };
        return control;
      }));
      assert.equal(controls.length, 2);
      assert.deepEqual(controls[0], controls[1]);
    }
  });
});

[1280, 390].forEach(width => {
  test(`card grids keep equal gutters at ${width}px`, async (t) => {
    const viewport = { width, height: 900 };
    const page = await browserPage(t, { viewport });
    await page.goto('/styleguide');
    const grids = page.locator('[aria-labelledby="theme-colors"] > div, .stats-grid, .metrics-grid, .efficiency-grid, .integration-grid, .savings-buckets');
    for (const grid of await grids.all()) await assertCardGutters(grid);
    const metrics = page.locator('[aria-labelledby="theme-metrics"]');
    const groups = metrics.locator('.stats-grid, .metrics-grid, .efficiency-grid');
    const bounds = await groups.evaluateAll(elements => elements.map(element => element.getBoundingClientRect().toJSON()));
    assert.equal(bounds[1].top - bounds[0].bottom, 12);
    assert.equal(bounds[2].top - bounds[1].bottom, 12);
  });
});

test('shared cards use consistent padding and related-text spacing', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  const cards = page.locator('[aria-labelledby="theme-colors"] .card, .stat-card, .metric-card, .efficiency-item, .integration-card, .integration-link, .savings-bucket, .savings-hero');
  for (const card of await cards.all()) await expect(card).toHaveCSS('padding', '16px');
  const labels = page.locator('.stat-label, .metric-label, .efficiency-label');
  const gaps = await labels.evaluateAll(elements => elements.map(element => {
    const label = element.getBoundingClientRect();
    const value = element.previousElementSibling.getBoundingClientRect();
    const gap = label.top - value.bottom;
    return gap;
  }));
  gaps.forEach(gap => assert.equal(gap, 4));
  await expect(page.locator('.integration-card').first()).toHaveCSS('row-gap', '12px');
});

test('ranked tool lists share the card spacing pattern', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  const section = page.locator('[aria-labelledby="theme-lists"]');
  await assertCardGutters(section.locator('.top-tools-list'));
  const card = await section.locator('.card').boundingBox();
  const list = await section.locator('.top-tools-list').boundingBox();
  assert.equal(list.y - card.y - card.height, 12);
});

test('pressing and selecting controls have distinct states', async (t) => {
  const page = await browserPage(t);
  await page.goto('/styleguide');
  const button = page.getByRole('button', { name: 'Example action', exact: true });
  await page.addStyleTag({ content: ':root { --accent-active: #123456; }' });
  await button.hover();
  await page.mouse.down();
  await expect(button).toHaveCSS('background-color', 'rgb(18, 52, 86)');
  await page.mouse.up();
  const group = page.getByRole('group', { name: 'Example integration state' });
  const selected = group.getByRole('button', { name: 'Enabled', exact: true });
  await expect(selected).toHaveAttribute('aria-pressed', 'true');
  await selected.click();
  await expect(selected).toHaveAttribute('aria-pressed', 'false');
});

test('auto-update selection follows its saved server state', async (t) => {
  const page = await browserPage(t);
  await page.goto('/plugins');
  const button = page.getByRole('button', { name: /Auto-Update:/ });
  const previous = await button.getAttribute('aria-pressed');
  await button.click();
  await page.waitForLoadState();
  await expect(button).toHaveAttribute('aria-pressed', String(previous !== 'true'));
  await page.reload();
  await expect(button).toHaveAttribute('aria-pressed', String(previous !== 'true'));
});

test('reduced motion disables transitions and status animations', async (t) => {
  const page = await browserPage(t, { reducedMotion: 'reduce' });
  await page.goto('/styleguide');
  await expect(page.getByRole('button', { name: 'Example action', exact: true })).toHaveCSS('transition-duration', '0s');
  await expect(page.locator('.flash-success')).toHaveCSS('animation-name', 'none');
  await expect(page.locator('.slider').first()).toHaveCSS('transition-duration', '0s');
});

async function browserPage(t: TestContext, options: BrowserContextOptions = {}): Promise<Page> {
  const context = await browser.newContext({ baseURL, ...options });
  t.after(() => context.close());
  const headers = { 'access-control-allow-origin': '*' };
  for (const [url, path] of scriptFixtures) {
    await context.route(url, route => route.fulfill({ path, contentType: 'text/javascript', headers }));
  }
  const body = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>';
  for (const url of logoSources) {
    await context.route(url, route => route.fulfill({ body, contentType: 'image/svg+xml', headers }));
  }
  const page = await context.newPage();
  return page;
}

async function assertIcons(icons: Locator) {
  await expect(icons).toHaveCount(5);
  const matches = await icons.evaluateAll(elements => elements.every(element => {
    const style = getComputedStyle(element);
    const correctSize = style.width === '14px' && style.height === '14px';
    const svgMask = style.maskImage.includes('data:image/svg+xml');
    const rendered = correctSize && svgMask;
    return rendered;
  }));
  assert.equal(matches, true);
}

async function selectFontSize(page: Page, size: string) {
  const picker = page.locator('[data-font-picker]');
  const open = await picker.getAttribute('open');
  if (open === null) await picker.locator('summary').click();
  const name = `${size}%`;
  await picker.getByRole('radio', { name, exact: true }).locator('..').click();
}

async function openSidebar(page: Page) {
  const closed = await page.locator('html').getAttribute('data-sidebar') === 'closed';
  if (closed) await page.getByRole('button', { name: 'Toggle sidebar' }).click();
}

async function assertSidebarLabels(page: Page) {
  const labels = await page.locator('.nav-item').evaluateAll(links => links.map(link => {
    const range = document.createRange();
    range.selectNodeContents(link.lastChild);
    const lines = range.getClientRects().length;
    const textRight = range.getBoundingClientRect().right;
    const linkRight = link.getBoundingClientRect().right;
    const padding = Number.parseFloat(getComputedStyle(link).paddingRight);
    const fits = textRight <= linkRight - padding + 0.5;
    const text = link.textContent.trim();
    const label = { text, lines, fits };
    return label;
  }));
  labels.forEach(label => {
    assert.equal(label.lines, 1, `${label.text} wraps`);
    assert.equal(label.fits, true, `${label.text} crowds the right padding`);
  });
}

async function assertSelectChevron(select: Locator) {
  await expect(select).toHaveCSS('appearance', 'none');
  await expect(select).toHaveCSS('background-image', /^url\("data:image\/svg\+xml,/);
  const themed = await select.evaluate(element => {
    const style = getComputedStyle(element);
    const muted = style.getPropertyValue('--text-muted').trim();
    const image = decodeURIComponent(style.backgroundImage);
    const matches = image.includes(`stroke="${muted}"`);
    return matches;
  });
  assert.equal(themed, true, 'chevron must use the current theme palette');
  await expect(select).toHaveCSS('background-repeat', 'no-repeat');
  await expect(select).toHaveCSS('padding-right', '40px');
}

async function assertCardGutters(grid: Locator) {
  const cards = await grid.locator(':scope > *').evaluateAll(elements => elements.map(element => element.getBoundingClientRect().toJSON()));
  cards.slice(1).forEach((card, index) => {
    const previous = cards[index];
    if (card.top === previous.top) {
      assert.equal(card.left - previous.right, 12, 'horizontal card gutter');
      return;
    }
    assert.equal(card.top - previous.bottom, 12, 'vertical card gutter');
  });
}
