// Phase 16 Plan 04 — Strategy 4 Configuration card static-contract tests
// (PG-OPS-09).
//
// CLAUDE.local.md npm lockdown forbids vitest / @testing-library/react.
// Node's --experimental-strip-types cannot process .tsx (JSX) files at the
// runner level for evaluation, but .tsx files used as test entry points can
// still be parsed. To be conservative, this test reads the ConfigCard
// source file as text and asserts contract invariants statically (matching
// the precedent set by web/src/components/Discovery/__tests__/
// DiscoverySection.test.ts).
//
// What this guarantees mechanically:
//   - ConfigCard.tsx exists and exports ConfigCard (named + default).
//   - 4 subsections render with their expected data-test markers.
//   - ENABLE-LIVE-CAPITAL toggle wires BreakerConfirmModal with the literal
//     phrase + the Phase 16 i18n keys.
//   - ENABLE-BREAKER toggle wires BreakerConfirmModal with the literal
//     phrase + the Phase 16 i18n keys.
//   - D-21 invariant: the live-capital and breaker-enabled POST bodies do
//     NOT include `operator_action` (typed-phrase modal IS the safety
//     mechanism; Plan 02 server guard is paper_mode-scoped only).
//   - Migrated gate fields (price_gap_free_bps, max_price_gap_bps,
//     enable_price_gap_gate, max_price_gap_pct) appear in the source.
//   - npm lockdown — no new dependencies.
//
// Run with:
//   cd /var/solana/data/arb/web && \
//     node --test --experimental-strip-types \
//       src/components/PriceGap/ConfigCard.test.tsx
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

import en from '../../i18n/en.ts';

const ROOT = process.cwd().endsWith('/web') ? process.cwd() : join(process.cwd(), 'web');
const SRC = join(ROOT, 'src/components/PriceGap/ConfigCard.tsx');
const PRICEGAP_PAGE = join(ROOT, 'src/pages/PriceGap.tsx');
const CONFIG_PAGE = join(ROOT, 'src/pages/Config.tsx');

function read(p: string): string {
  return readFileSync(p, 'utf8');
}

test('ConfigCard.tsx exists', () => {
  assert.ok(existsSync(SRC), `missing: ${SRC}`);
});

test('ConfigCard exports the ConfigCard symbol (named + default)', () => {
  const src = read(SRC);
  assert.ok(/export const ConfigCard\b/.test(src), 'no named export');
  assert.ok(/export default ConfigCard/.test(src), 'no default export');
});

test('ConfigCard renders 4 subsections', () => {
  const src = read(SRC);
  for (const marker of [
    'pg-config-section-scanner',
    'pg-config-section-breaker',
    'pg-config-section-ramp',
    'pg-config-section-live-capital',
  ]) {
    assert.ok(src.includes(marker), `missing data-test marker: ${marker}`);
  }
});

test('ConfigCard references all 4 subsection heading i18n keys', () => {
  const src = read(SRC);
  for (const k of [
    'pricegap.config.section.scanner',
    'pricegap.config.section.breaker',
    'pricegap.config.section.ramp',
    'pricegap.config.section.liveCapital',
  ]) {
    assert.ok(src.includes(k), `missing i18n key: ${k}`);
    assert.ok(k in (en as Record<string, string>), `i18n key not in en.ts: ${k}`);
  }
});

test('ConfigCard wires ENABLE-LIVE-CAPITAL via BreakerConfirmModal (literal phrase)', () => {
  const src = read(SRC);
  assert.ok(
    src.includes('magicPhrase="ENABLE-LIVE-CAPITAL"'),
    'missing literal ENABLE-LIVE-CAPITAL phrase prop',
  );
  assert.ok(
    src.includes('pricegap.config.confirmPrompt.enableLiveCapital'),
    'missing enableLiveCapital prompt key',
  );
  assert.ok(
    src.includes('pricegap.config.confirmTitle.enableLiveCapital'),
    'missing enableLiveCapital title key',
  );
});

test('ConfigCard wires ENABLE-BREAKER via BreakerConfirmModal (literal phrase)', () => {
  const src = read(SRC);
  assert.ok(
    src.includes('magicPhrase="ENABLE-BREAKER"'),
    'missing literal ENABLE-BREAKER phrase prop',
  );
  assert.ok(
    src.includes('pricegap.config.confirmPrompt.enableBreaker'),
    'missing enableBreaker prompt key',
  );
  assert.ok(
    src.includes('pricegap.config.confirmTitle.enableBreaker'),
    'missing enableBreaker title key',
  );
});

test('D-21 invariant: live-capital postConfig() argument literal excludes operator_action', () => {
  const src = read(SRC);
  const fnSlice = src.match(/submitLiveCapital[\s\S]+?setConfig/);
  assert.ok(fnSlice, 'submitLiveCapital function body not found');
  const body = fnSlice[0];
  const postLine = body.match(/await postConfig\(([^;]+)\);/);
  assert.ok(postLine, 'live-capital postConfig call not found');
  assert.ok(postLine[1].includes('price_gap: { live_capital: next }'), 'live-capital POST must use nested price_gap.live_capital');
  assert.ok(
    !postLine[1].includes('operator_action'),
    `D-21 violated: live-capital POST path includes operator_action: ${postLine[1]}`,
  );
});

test('D-21 invariant: breaker-enabled postConfig() argument literal excludes operator_action', () => {
  const src = read(SRC);
  const fnSlice = src.match(/submitBreaker[\s\S]+?setConfig/);
  assert.ok(fnSlice, 'submitBreaker function body not found');
  const body = fnSlice[0];
  const postLine = body.match(/await postConfig\(([^;]+)\);/);
  assert.ok(postLine, 'breaker postConfig call not found');
  assert.ok(postLine[1].includes('price_gap: { breaker_enabled: next }'), 'breaker POST must use nested price_gap.breaker_enabled');
  assert.ok(
    !postLine[1].includes('operator_action'),
    `D-21 violated: breaker POST path includes operator_action: ${postLine[1]}`,
  );
});

test('Migrated gate fields are present in ConfigCard', () => {
  const src = read(SRC);
  for (const k of [
    'price_gap_free_bps',
    'max_price_gap_bps',
    'enable_price_gap_gate',
    'max_price_gap_pct',
  ]) {
    assert.ok(src.includes(k), `migrated gate field missing: ${k}`);
  }
});

test('PriceGap.tsx imports ConfigCard from PriceGap subtree', () => {
  const src = read(PRICEGAP_PAGE);
  assert.ok(
    src.includes("from '../components/PriceGap/ConfigCard"),
    'PriceGap.tsx does not import ConfigCard',
  );
  assert.ok(
    src.includes('<ConfigCard'),
    'PriceGap.tsx does not render <ConfigCard />',
  );
});

test('PriceGap.tsx togglePaper sends operator_action: true (PG-FIX-02)', () => {
  const src = read(PRICEGAP_PAGE);
  // Locate the togglePaper function body.
  const fn = src.match(/const togglePaper = useCallback\([\s\S]+?postConfig\(\{[^}]+\}\)/);
  assert.ok(fn, 'togglePaper function not found');
  const body = fn[0];
  assert.ok(
    body.includes('price_gap_paper_mode'),
    'togglePaper does not write price_gap_paper_mode',
  );
  assert.ok(
    body.includes('operator_action: true'),
    'PG-FIX-02 regression: togglePaper missing operator_action: true',
  );
});

test('Config.tsx legacy controls are deleted (no live references)', () => {
  const src = read(CONFIG_PAGE);
  for (const k of [
    'price_gap_free_bps',
    'max_price_gap_bps',
    'enable_price_gap_gate',
    'max_price_gap_pct',
  ]) {
    assert.ok(
      !src.includes(k),
      `Config.tsx still references legacy key: ${k}`,
    );
  }
});

test('RampReconcileSection still in PriceGap.tsx', () => {
  const src = read(PRICEGAP_PAGE);
  assert.ok(
    src.includes('RampReconcileSection'),
    'RampReconcileSection removed from PriceGap.tsx',
  );
});

test('ConfigCard no longer points operators to CLI or config file edits', () => {
  const src = read(SRC);
  assert.ok(!/pg-admin/i.test(src), 'ConfigCard still references pg-admin');
  assert.ok(!/config\.json/i.test(src), 'ConfigCard still references config.json');
});

test('localStorage key for expanded state matches UI-SPEC', () => {
  const src = read(SRC);
  assert.ok(
    src.includes('arb_pg_config_expanded'),
    'localStorage key arb_pg_config_expanded missing',
  );
});

test('Card outer uses card-surface + spacing per UI-SPEC §Spacing Scale', () => {
  const src = read(SRC);
  assert.ok(src.includes('card-surface'), 'card-surface class missing');
  // Check at least one of the canonical spacing tokens (p-4 mt-4 mb-6).
  assert.ok(/className=".*card-surface.*p-4/.test(src), 'p-4 padding missing');
});

test('Magic strings ENABLE-LIVE-CAPITAL / ENABLE-BREAKER appear LITERAL on magicPhrase prop', () => {
  const src = read(SRC);
  // The literal magic phrase must be passed verbatim to the
  // BreakerConfirmModal magicPhrase prop. Other locations (lowercase modal
  // kind discriminators like 'enable-live-capital' that are routing tags,
  // not user-facing copy) are OK and intentional.
  assert.ok(
    src.includes('magicPhrase="ENABLE-LIVE-CAPITAL"'),
    'magicPhrase="ENABLE-LIVE-CAPITAL" prop missing',
  );
  assert.ok(
    src.includes('magicPhrase="ENABLE-BREAKER"'),
    'magicPhrase="ENABLE-BREAKER" prop missing',
  );
  // The magic phrases must NOT be lowercased in any user-visible string —
  // the lowercase variants in the source can ONLY appear as routing
  // discriminators wrapped in single quotes (e.g. 'enable-live-capital' as
  // a kind tag). Verify by ensuring lowercase forms never appear inside a
  // SINGLE-LINE double-quoted string (UI copy / JSX props).
  assert.ok(
    !/"[^"\n]*enable-live-capital[^"\n]*"/.test(src),
    'lowercase enable-live-capital leaked into a double-quoted string',
  );
  assert.ok(
    !/"[^"\n]*enable-breaker[^"\n]*"/.test(src),
    'lowercase enable-breaker leaked into a double-quoted string',
  );
});
