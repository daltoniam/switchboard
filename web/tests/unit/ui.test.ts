import assert from 'node:assert/strict';
import { afterEach, mock, test } from 'node:test';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import { UI } from '../../static/ui.mjs';

const ui = UI.getInstance();
const layout = readFileSync(new URL('../../templates/layouts/base.templ', import.meta.url), 'utf8');
afterEach(() => ui.destroy());

test('stylesheet generation does not download the logo catalog or SVGs', () => {
  const source = readFileSync(new URL('../../style.js', import.meta.url), 'utf8');
  assert.doesNotMatch(source, /\bfetch\s*\(/);
});

test('branded logo CSS uses immutable CDN URLs and preserves theme variants', () => {
  const css = readFileSync(new URL('../../static/style.css', import.meta.url), 'utf8');
  const rules = [...css.matchAll(/\.integration-icon\[data-integration="([^"]+)"\]\{([^}]+)\}/g)];
  const branded = rules.filter(([, , rule]) => rule.includes('--integration-light:'));
  assert.equal(rules.length, 60);
  assert.equal(branded.length, 33);
  branded.forEach(([, name, rule]) => {
    assert.match(rule, /--integration-light:url\("https:\/\/cdn\.jsdelivr\.net\/gh\/pheralb\/svgl@[a-f0-9]{40}\/static\/library\/[\w.-]+\.svg"\)/, name);
  });
  const entries = branded.map(([, name, rule]) => {
    const entry = [name, rule];
    return entry;
  });
  const brandedRules = new Map(entries);
  ['aws', 'github', 'ollama', 'vercel', 'x'].forEach(name => {
    const rule = brandedRules.get(name);
    assert.match(rule, /--integration-dark:url\("https:\/\/cdn\.jsdelivr\.net\/gh\/pheralb\/svgl@[a-f0-9]{40}\/static\/library\/[\w.-]+\.svg"\)/, name);
  });
});

function restoreFixture(saved = {}, dark = false) {
  const dataset = {};
  const documentElement = { dataset };
  const themes = [{ id: 'github-light' }, { id: 'github-dark' }, { id: 'nord' }];
  const defaults = { light: 'github-light', dark: 'github-dark' };
  const fontSizes = ['80', '100', '120'];
  const textContent = JSON.stringify({ themes, defaults, fontSizes });
  const document = { documentElement, getElementById: () => ({ textContent }) };
  const localStorage = { getItem: key => saved[key] ?? null };
  const matchMedia = query => {
    const matches = query === '(prefers-color-scheme: dark)' && dark;
    const result = { matches };
    return result;
  };
  const state = { document, localStorage, matchMedia, dataset };
  return state;
}

function runRestore(state) {
  const script = layout.match(/<script id="appearance-bootstrap">([\s\S]*?)<\/script>/)?.[1];
  assert.ok(script, 'restore preferences in the head before rendering');
  runInNewContext(script, state);
}

[false, true].forEach(dark => {
  test(`first visit uses the system appearance (dark=${dark})`, () => {
    const state = restoreFixture({}, dark);
    runRestore(state);
    const expected = dark ? 'github-dark' : 'github-light';
    assert.equal(state.dataset.theme, expected);
    assert.equal(state.dataset.fontSize, '100');
  });
});

test('saved appearance overrides the system before UI initialization', () => {
  const state = restoreFixture({ 'switchboard.theme': 'nord', 'switchboard.fontSize': '120' });
  runRestore(state);
  assert.deepEqual(state.dataset, { theme: 'nord', fontSize: '120' });
});

test('unknown preferences fall back to supported defaults', () => {
  const state = restoreFixture({ 'switchboard.theme': 'removed-theme', 'switchboard.fontSize': '999' });
  runRestore(state);
  assert.deepEqual(state.dataset, { theme: 'github-light', fontSize: '100' });
});

test('denied storage still applies the system default', () => {
  const state = restoreFixture({}, true);
  state.localStorage.getItem = () => { throw new Error('Storage denied'); };
  runRestore(state);
  assert.deepEqual(state.dataset, { theme: 'github-dark', fontSize: '100' });
});

test('bootstrap leaves navigation visible until its controls initialize', () => {
  const state = restoreFixture({ 'switchboard.sidebar': 'closed' });
  state.matchMedia = () => ({ matches: true });
  runRestore(state);
  assert.equal(state.dataset.sidebar, undefined);
});

function appearanceFixture() {
  const document = new EventTarget();
  const preferences = { theme: 'nord', fontSize: '100' };
  document.documentElement = { dataset: preferences };
  const radios = preferenceRadios('theme', ['nord', 'github-light']);
  const findAll = preferenceElements(radios);
  const htmx = { find: () => null, closest: () => null, findAll };
  const setItem = mock.fn();
  const localStorage = { setItem };
  const state = { document, htmx, localStorage, radios };
  return state;
}

test('appearance controls sync, persist changes, and stop listening after destroy', () => {
  const state = appearanceFixture();
  const [selected, control] = state.radios;
  ui.init(state);
  assert.deepEqual(state.radios.map(radio => radio.value), ['nord', 'github-light']);
  assert.deepEqual(state.radios.map(radio => radio.checked), [true, false]);
  assert.ok(state.radios.every(radio => !radio.disabled));
  selected.checked = false;
  control.checked = true;
  const event = new Event('change');
  Object.defineProperty(event, 'target', { value: control });
  state.document.dispatchEvent(event);
  assert.equal(state.document.documentElement.dataset.theme, 'github-light');
  assert.deepEqual(state.localStorage.setItem.mock.calls[0].arguments, ['switchboard.theme', 'github-light']);
  ui.destroy();
  state.document.dispatchEvent(event);
  assert.equal(state.localStorage.setItem.mock.callCount(), 1);
});

test('denied storage does not prevent changing appearance', () => {
  const state = appearanceFixture();
  const [selected, target] = state.radios;
  state.localStorage.setItem = () => { throw new Error('Storage denied'); };
  ui.init(state);
  selected.checked = false;
  target.checked = true;
  ui.handleEvent({ type: 'change', target });
  assert.equal(state.document.documentElement.dataset.theme, 'github-light');
});

test('font-size radios retain their values and restore exactly one selection', () => {
  const state = appearanceFixture();
  const radios = preferenceRadios('fontSize', ['80', '100', '120']);
  state.document.documentElement.dataset.fontSize = '120';
  state.htmx.findAll = preferenceElements(radios);
  ui.init(state);
  assert.deepEqual(radios.map(radio => radio.value), ['80', '100', '120']);
  assert.deepEqual(radios.map(radio => radio.checked), [false, false, true]);
  assert.ok(radios.every(radio => !radio.disabled));
});

test('font-size changes persist only the checked radio', () => {
  const state = appearanceFixture();
  const radios = preferenceRadios('fontSize', ['80', '100', '120']);
  state.htmx.findAll = preferenceElements(radios);
  ui.init(state);
  const [target] = radios;
  ui.handleEvent({ type: 'change', target });
  assert.equal(state.localStorage.setItem.mock.callCount(), 0);
  target.checked = true;
  ui.handleEvent({ type: 'change', target });
  assert.equal(state.document.documentElement.dataset.fontSize, '80');
  assert.deepEqual(state.localStorage.setItem.mock.calls[0].arguments, ['switchboard.fontSize', '80']);
});

function preferenceRadios(preference, values) {
  const radios = values.map(value => {
    const dataset = { preference };
    const radio = { type: 'radio', dataset, value, checked: false, disabled: true, matches: () => true };
    return radio;
  });
  return radios;
}

function preferenceElements(controls) {
  const findAll = (root, selector) => {
    if (selector.includes('[data-preference')) return controls;
    const elements = [];
    return elements;
  };
  return findAll;
}

test('getInstance returns the same instance and direct construction is rejected', () => {
  assert.equal(UI.getInstance(), ui);
  assert.throws(() => new UI(), /Use UI.getInstance/);
});

test('initialization without HTMX can be retried later', () => {
  ui.init({});
  const state = refreshFixture();
  ui.init(state);
  state.fire('htmx:beforeRequest');
  assert.equal(state.form.getAttribute('aria-busy'), 'true');
});


['success', 'error'].forEach((key) => {
  test(`dismiss ${key} without changing other URL state`, () => {
    const state = fixture(key);
    ui.init(state).dismissFlash(state.banner);
    const [call] = state.history.replaceState.mock.calls;
    const [saved, title, href] = call.arguments;
    const url = new URL(href);
    assert.equal(url.searchParams.has(key), false);
    assert.equal(url.searchParams.get('filter'), 'enabled');
    assert.equal(url.searchParams.size, 2);
    assert.equal(url.hash, '#connected');
    assert.deepEqual(saved, { page: 2 });
    assert.equal(title, '');
    assert.equal(state.banner.remove.mock.callCount(), 1);
    assert.equal(state.heading.focus.mock.callCount(), 1);
  });
});

test('missing banners are harmless', () => {
  const state = fixture();
  ui.init(state).dismissFlash(null);
  assert.equal(state.history.replaceState.mock.callCount(), 0);
  assert.equal(state.banner.remove.mock.callCount(), 0);
  assert.equal(state.heading.focus.mock.callCount(), 0);
});

test('initialization registers once and ignores unrelated clicks', () => {
  const document = new EventTarget();
  const htmx = { closest: () => null, find: () => null, findAll: () => [] };
  const environment = { document, htmx };
  let registrations = 0;
  const add = document.addEventListener.bind(document);
  document.addEventListener = (...args) => { registrations += 1; add(...args); };
  ui.init(environment);
  const initialRegistrations = registrations;
  assert.equal(ui.init(environment), ui);
  const event = new Event('click');
  document.dispatchEvent(event);
  assert.equal(registrations, initialRegistrations);
  ui.destroy();
  ui.init(environment);
  assert.equal(registrations, initialRegistrations * 2);
});

test('clicking inside a dismiss button removes only its banner', () => {
  const state = fixture();
  const icon = {};
  const button = {};
  const { banner } = state;
  state.htmx.closest = (element, selector) => {
    const clickedIcon = element === icon && selector === '[data-dismiss-flash]';
    const dismissButton = element === button && selector === '[data-flash]';
    if (clickedIcon) return button;
    if (dismissButton) return banner;
    return null;
  };
  ui.init(state);
  const event = new Event('click');
  Object.defineProperty(event, 'target', { value: icon });
  state.document.dispatchEvent(event);
  assert.equal(state.banner.remove.mock.callCount(), 1);
  assert.equal(state.heading.focus.mock.callCount(), 1);
  ui.destroy();
  state.document.dispatchEvent(event);
  assert.equal(state.banner.remove.mock.callCount(), 1);
});

function refreshFixture() {
  const environment = fixture('refresh');
  const { document, htmx, banner } = environment;
  const toggle = mock.fn();
  const classList = { toggle };
  Object.assign(banner, attributeFixture(), { hidden: true, classList });
  const message = { textContent: '' };
  const button = { disabled: false };
  const selectors = new Set(['#page-refresh', '#page-refresh, #health-refresh']);
  const matches = selectors.has.bind(selectors);
  const form = { ...attributeFixture(), matches };
  htmx.find = (root, selector) => {
    if (root === form) return button;
    if (root === banner) return message;
    if (selector === '#page-refresh-status') return banner;
    return null;
  };
  const fire = refreshEvents(document, form);
  const state = { ...environment, message, button, form, fire };
  return state;
}

function fixture(key = 'success') {
  const replaceState = mock.fn();
  const focus = mock.fn();
  const remove = mock.fn();
  const location = { href: 'https://example.test/integrations?success=Saved&error=Failed&filter=enabled#connected' };
  const state = { page: 2 };
  const dataset = { flash: key };
  const history = { state, replaceState };
  const heading = { focus };
  const banner = { dataset, remove, contains: () => false };
  const document = new EventTarget();
  const find = (root, selector) => {
    if (selector.endsWith(', main')) return heading;
    return null;
  };
  const htmx = { find, findAll: () => [], closest: () => null };
  const environment = { location, history, document, htmx, banner, heading };
  return environment;
}

function attributeFixture() {
  const attributes = new Map();
  const setAttribute = attributes.set.bind(attributes);
  const getAttribute = name => attributes.get(name) ?? null;
  const removeAttribute = attributes.delete.bind(attributes);
  const attributesAPI = { setAttribute, getAttribute, removeAttribute };
  return attributesAPI;
}

function refreshEvents(document, form) {
  const fire = (name, detail = {}) => {
    const event = new Event(name, { cancelable: true });
    event.detail = { elt: form, ...detail };
    document.dispatchEvent(event);
    return event;
  };
  return fire;
}

test('announces completion after the swap without changing HTMX control state', t => {
  const apis = ['setTimeout'];
  t.mock.timers.enable({ apis });
  const state = refreshFixture();
  ui.init(state);
  state.fire('htmx:beforeRequest');
  assert.equal(state.button.disabled, false);
  assert.equal(state.form.getAttribute('aria-busy'), 'true');
  assert.equal(state.banner.hidden, true);
  state.button.disabled = true;
  state.fire('htmx:afterSwap');
  assert.equal(state.message.textContent, 'Refreshed.');
  assert.equal(state.banner.hidden, false);
  assert.equal(state.button.disabled, true);
  state.button.disabled = false;
  state.fire('htmx:afterRequest', { successful: true });
  assert.equal(state.banner.hidden, false);
  t.mock.timers.tick(3000);
  assert.equal(state.banner.hidden, true);
  assert.equal(state.button.disabled, false);
  assert.equal(state.form.getAttribute('aria-busy'), 'false');
});

test('cancelled refresh clears busy state without reporting an error', () => {
  const state = refreshFixture();
  ui.init(state);
  state.fire('htmx:beforeRequest');
  state.fire('htmx:afterRequest');
  state.fire('htmx:sendAbort');
  assert.equal(state.button.disabled, false);
  assert.equal(state.form.getAttribute('aria-busy'), 'false');
  assert.equal(state.banner.hidden, true);
});

['htmx:sendError', 'htmx:timeout', 'htmx:responseError', 'htmx:swapError'].forEach((name) => {
  test(`${name} reports failure and clears the message on retry`, () => {
    const state = refreshFixture();
    ui.init(state);
    state.fire('htmx:beforeRequest');
    state.fire(name);
    assert.equal(state.message.textContent, 'Could not refresh. Try again.');
    assert.equal(state.banner.getAttribute('role'), 'alert');
    assert.equal(state.button.disabled, false);
    assert.equal(state.form.getAttribute('aria-busy'), 'false');
    state.fire('htmx:beforeRequest');
    assert.equal(state.banner.hidden, true);
    assert.equal(state.form.getAttribute('aria-busy'), 'true');
  });
});

test('network completion waits for the specific error event before reporting failure', () => {
  const state = refreshFixture();
  ui.init(state);
  state.fire('htmx:beforeRequest');
  state.fire('htmx:afterRequest');
  assert.equal(state.banner.hidden, true);
  state.fire('htmx:sendError');
  assert.equal(state.message.textContent, 'Could not refresh. Try again.');
  assert.equal(state.banner.hidden, false);
});

test('initialization is idempotent and listeners can be removed', () => {
  const state = refreshFixture();
  ui.init(state);
  assert.equal(ui.init(state), ui);
  const elt = { matches: () => false };
  state.fire('htmx:beforeRequest', { elt });
  assert.equal(state.button.disabled, false);
  ui.destroy();
  state.fire('htmx:beforeRequest');
  assert.equal(state.form.getAttribute('aria-busy'), null);
});

test('destroy releases the old document before reinitializing', () => {
  const previous = refreshFixture();
  ui.init(previous);
  ui.destroy();
  ui.destroy();
  const current = refreshFixture();
  ui.init(current);
  previous.fire('htmx:beforeRequest');
  current.fire('htmx:beforeRequest');
  assert.equal(previous.form.getAttribute('aria-busy'), null);
  assert.equal(current.form.getAttribute('aria-busy'), 'true');
  assert.equal(UI.getInstance(), ui);
});
