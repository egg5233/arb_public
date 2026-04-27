// Phase 999.1 Plan 05 — i18n parity regression test.
//
// Asserts that en.ts and zh-TW.ts have identical key sets, with explicit
// coverage for the new pricegap.candidates.modal.direction.* namespace
// added by Plan 03. Catches translation drift where a key is added to one
// locale and forgotten in the other (operator-confusion threat T-999.1-05-03).
//
// Run with: node --test --experimental-strip-types src/i18n/parity.test.ts
// Requires Node 22+ (per CLAUDE.local.md npm lockdown — no Vitest installable).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import en from './en.ts';
import zhTW from './zh-TW.ts';

test('en and zh-TW have identical total key counts', () => {
  const enKeys = Object.keys(en);
  const zhKeys = Object.keys(zhTW);
  assert.equal(
    enKeys.length,
    zhKeys.length,
    `key count drift: en=${enKeys.length}, zh-TW=${zhKeys.length}`,
  );
});

test('every en key exists in zh-TW', () => {
  const missing: string[] = [];
  for (const k of Object.keys(en)) {
    if (!(k in zhTW)) missing.push(k);
  }
  assert.equal(missing.length, 0, `missing in zh-TW: ${missing.join(', ')}`);
});

test('every zh-TW key exists in en', () => {
  const enRec = en as Record<string, string>;
  const missing: string[] = [];
  for (const k of Object.keys(zhTW)) {
    if (!(k in enRec)) missing.push(k);
  }
  assert.equal(missing.length, 0, `missing in en: ${missing.join(', ')}`);
});

test('pricegap.candidates.modal.direction.* namespace has 5 keys in both en and zh-TW', () => {
  const prefix = 'pricegap.candidates.modal.direction';
  const enDirKeys = Object.keys(en).filter((k) => k.startsWith(prefix));
  const zhDirKeys = Object.keys(zhTW).filter((k) => k.startsWith(prefix));
  assert.equal(
    enDirKeys.length,
    5,
    `en direction keys: ${enDirKeys.length} (expected 5)`,
  );
  assert.equal(
    zhDirKeys.length,
    5,
    `zh-TW direction keys: ${zhDirKeys.length} (expected 5)`,
  );
  for (const k of enDirKeys) {
    assert.ok(k in zhTW, `direction key missing in zh-TW: ${k}`);
  }
});

test('pricegap.candidates.modal.direction.* required keys all present', () => {
  const required = [
    'pricegap.candidates.modal.direction.label',
    'pricegap.candidates.modal.direction.option.pinned',
    'pricegap.candidates.modal.direction.option.bidirectional',
    'pricegap.candidates.modal.direction.help',
    'pricegap.candidates.modal.direction.validation.invalid',
  ];
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  for (const k of required) {
    assert.ok(k in enRec, `required key missing in en: ${k}`);
    assert.ok(k in zhRec, `required key missing in zh-TW: ${k}`);
    assert.ok(enRec[k]!.length > 0, `empty en value for ${k}`);
    assert.ok(zhRec[k]!.length > 0, `empty zh-TW value for ${k}`);
  }
});
