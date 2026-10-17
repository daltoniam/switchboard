import assert from 'node:assert/strict';
import { test } from 'node:test';
import { readFileSync } from 'node:fs';

test('stylesheet generation does not download the logo catalog or SVGs', () => {
  const source = readFileSync(new URL('../../style.js', import.meta.url), 'utf8');
  assert.doesNotMatch(source, /\bfetch\s*\(/);
});

test('branded logo CSS uses immutable CDN URLs and preserves theme variants', () => {
  const css = readFileSync(new URL('../../static/style.css', import.meta.url), 'utf8');
  const rules = [...css.matchAll(/\.integration-icon\[data-integration="([^"]+)"\]\{([^}]+)\}/g)];
  const branded = rules.filter(([, , rule]) => rule.includes('--integration-light:'));
  assert.equal(rules.length, 59);
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

