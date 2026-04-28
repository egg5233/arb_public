// Phase 11 Plan 06 — Discovery i18n lockstep regression test.
//
// Asserts that the new pricegap.discovery.* namespace is in lockstep across
// en.ts and zh-TW.ts: identical key sets, no empty values, and consistent
// {n}/{exchanges} token interpolation.  Catches the same translation drift
// that parity.test.ts catches at file scope but with namespace-scoped
// assertions and per-key token-token verification.
//
// Threat model: T-11-38 (i18n drift exposes English fallback to zh-TW users)
// is mitigated when this test runs in CI alongside parity.test.ts.
//
// Run with: node --test --experimental-strip-types src/i18n/__tests__/discovery_keys.test.ts
// Requires Node 22+ (per CLAUDE.local.md npm lockdown — no Vitest installable).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import en from '../en.ts';
import zhTW from '../zh-TW.ts';

const NAMESPACE = 'pricegap.discovery.';

function discoveryKeys(record: Record<string, string>): string[] {
  return Object.keys(record).filter((k) => k.startsWith(NAMESPACE));
}

test('pricegap.discovery.* key sets are identical between en and zh-TW', () => {
  const enKeys = new Set(discoveryKeys(en as Record<string, string>));
  const zhKeys = new Set(discoveryKeys(zhTW as Record<string, string>));
  const missingInZh: string[] = [];
  for (const k of enKeys) {
    if (!zhKeys.has(k)) missingInZh.push(k);
  }
  const missingInEn: string[] = [];
  for (const k of zhKeys) {
    if (!enKeys.has(k)) missingInEn.push(k);
  }
  assert.equal(
    missingInZh.length,
    0,
    `discovery keys missing in zh-TW: ${missingInZh.join(', ')}`,
  );
  assert.equal(
    missingInEn.length,
    0,
    `discovery keys missing in en: ${missingInEn.join(', ')}`,
  );
});

test('every pricegap.discovery.* value is non-empty in both en and zh-TW', () => {
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  for (const k of discoveryKeys(enRec)) {
    assert.ok(
      enRec[k] != null && enRec[k]!.length > 0,
      `empty en value for ${k}`,
    );
    assert.ok(
      zhRec[k] != null && zhRec[k]!.length > 0,
      `empty zh-TW value for ${k}`,
    );
  }
});

test('token interpolation is consistent: {n} and {exchanges} appear in both locales when they appear in en', () => {
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  const tokenRegex = /\{(n|exchanges)\}/g;
  for (const k of discoveryKeys(enRec)) {
    const enValue = enRec[k]!;
    const zhValue = zhRec[k]!;
    const enTokens = new Set(enValue.match(tokenRegex) ?? []);
    const zhTokens = new Set(zhValue.match(tokenRegex) ?? []);
    for (const tok of enTokens) {
      assert.ok(
        zhTokens.has(tok),
        `${k}: en value contains ${tok} but zh-TW value does not (en="${enValue}", zh-TW="${zhValue}")`,
      );
    }
    for (const tok of zhTokens) {
      assert.ok(
        enTokens.has(tok),
        `${k}: zh-TW value contains ${tok} but en value does not`,
      );
    }
  }
});

test('discovery namespace key count is at the expected floor (≥30) — guards against accidental key deletion', () => {
  const enKeys = discoveryKeys(en as Record<string, string>);
  // UI-SPEC enumerates ≥30 keys (5 header + 2 disabled banner + 2 error banner +
  // 9 cycle stats + 7 score history + 13 why-rejected + 2 timeline = ~40).
  assert.ok(
    enKeys.length >= 30,
    `expected ≥30 pricegap.discovery.* keys, got ${enKeys.length}`,
  );
});

test('all required reason-code keys are present', () => {
  const required = [
    'pricegap.discovery.whyRejected.reason.insufficient_persistence',
    'pricegap.discovery.whyRejected.reason.stale_bbo',
    'pricegap.discovery.whyRejected.reason.insufficient_depth',
    'pricegap.discovery.whyRejected.reason.denylist',
    'pricegap.discovery.whyRejected.reason.bybit_blackout',
    'pricegap.discovery.whyRejected.reason.symbol_not_listed_long',
    'pricegap.discovery.whyRejected.reason.symbol_not_listed_short',
    'pricegap.discovery.whyRejected.reason.sample_error',
  ];
  const enRec = en as Record<string, string>;
  const zhRec = zhTW as Record<string, string>;
  for (const k of required) {
    assert.ok(k in enRec, `required reason key missing in en: ${k}`);
    assert.ok(k in zhRec, `required reason key missing in zh-TW: ${k}`);
  }
});
