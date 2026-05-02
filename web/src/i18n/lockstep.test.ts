// Phase 16 Plan 04 — i18n lockstep regression test (PG-OPS-09 D-20).
//
// Asserts that en.ts and zh-TW.ts have identical key sets, with explicit
// coverage for the new pricegap.config.* namespace added by this plan.
// Closes Pitfall 6 — translation drift between locale files.
//
// CLAUDE.local.md npm lockdown forbids installing vitest. This test uses
// Node's built-in test runner (Node 22+ ships with `node --test`).
//
// Run with:
//   cd /var/solana/data/arb/web && \
//     node --test --experimental-strip-types src/i18n/lockstep.test.ts
import { test } from 'node:test';
import assert from 'node:assert/strict';

import en from './en.ts';
import zhTW from './zh-TW.ts';

test('en and zh-TW have identical key sets (Phase 16 PG-OPS-09 D-20)', () => {
  const enKeys = Object.keys(en).sort();
  const zhKeys = Object.keys(zhTW).sort();
  const enOnly = enKeys.filter((k) => !zhKeys.includes(k));
  const zhOnly = zhKeys.filter((k) => !enKeys.includes(k));
  assert.deepEqual(
    enOnly,
    [],
    `keys present in en but missing in zh-TW: ${enOnly.join(', ')}`,
  );
  assert.deepEqual(
    zhOnly,
    [],
    `keys present in zh-TW but missing in en: ${zhOnly.join(', ')}`,
  );
});

test('pricegap.config.* namespace key parity (Phase 16 PG-OPS-09)', () => {
  const prefix = 'pricegap.config.';
  const enConfigKeys = Object.keys(en).filter((k) => k.startsWith(prefix));
  const zhConfigKeys = Object.keys(zhTW).filter((k) => k.startsWith(prefix));
  assert.equal(
    enConfigKeys.length,
    zhConfigKeys.length,
    `pricegap.config.* drift: en=${enConfigKeys.length}, zh-TW=${zhConfigKeys.length}`,
  );
  assert.ok(
    enConfigKeys.length >= 50,
    `expected at least 50 pricegap.config.* keys, got ${enConfigKeys.length}`,
  );
  for (const k of enConfigKeys) {
    assert.ok(k in zhTW, `pricegap.config.* key missing in zh-TW: ${k}`);
  }
});

test('all pricegap.config.* values are non-empty in both locales', () => {
  const prefix = 'pricegap.config.';
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  for (const k of Object.keys(enRec)) {
    if (!k.startsWith(prefix)) continue;
    assert.ok(enRec[k] && enRec[k].length > 0, `empty en value for ${k}`);
    assert.ok(zhRec[k] && zhRec[k].length > 0, `empty zh-TW value for ${k}`);
  }
});

test('magic strings ENABLE-LIVE-CAPITAL / ENABLE-BREAKER appear LITERAL in both locales (D-18)', () => {
  // The magic phrases must appear verbatim in the prompt strings of both
  // locales — never translated, never lowercased, never split by spaces.
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  const liveCapEn = enRec['pricegap.config.confirmPrompt.enableLiveCapital'];
  const liveCapZh = zhRec['pricegap.config.confirmPrompt.enableLiveCapital'];
  const breakerEn = enRec['pricegap.config.confirmPrompt.enableBreaker'];
  const breakerZh = zhRec['pricegap.config.confirmPrompt.enableBreaker'];
  assert.ok(liveCapEn.includes('ENABLE-LIVE-CAPITAL'), 'EN prompt missing ENABLE-LIVE-CAPITAL');
  assert.ok(liveCapZh.includes('ENABLE-LIVE-CAPITAL'), 'zh-TW prompt missing ENABLE-LIVE-CAPITAL');
  assert.ok(breakerEn.includes('ENABLE-BREAKER'), 'EN prompt missing ENABLE-BREAKER');
  assert.ok(breakerZh.includes('ENABLE-BREAKER'), 'zh-TW prompt missing ENABLE-BREAKER');
  // Negative — neither locale should use the lowercased form.
  for (const v of [liveCapEn, liveCapZh, breakerEn, breakerZh]) {
    assert.ok(!v.includes('enable-live-capital'), 'lowercase variant leaked into prompt');
    assert.ok(!v.includes('enable-breaker'), 'lowercase variant leaked into prompt');
  }
});
