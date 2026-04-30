// Phase 11 Plan 06 — Discovery component static-contract tests.
//
// CLAUDE.local.md npm lockdown forbids vitest / @testing-library/react.
// Node's --experimental-strip-types cannot process .tsx (JSX) files, so
// this test file is .ts and uses filesystem + regex contract checks
// instead of mounting React.  The visual checkpoint (Task 3) covers
// behavioral correctness.
//
// What this guarantees mechanically:
//   - All 6 component files + 1 hook exist on disk.
//   - Each component file exports its named symbol.
//   - Each component imports useLocale and the i18n keys referenced are
//     defined in en.ts (TypeScript compile catches missing keys at build).
//   - Hook contains the documented API endpoints + WS event types.
//   - npm lockdown — no new dependencies imported.
//
// Run with: node --test --experimental-strip-types
//   src/components/Discovery/__tests__/DiscoverySection.test.ts
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';

import en from '../../../i18n/en.ts';

// Resolve the Discovery directory relative to this test file.  process.cwd()
// is the project root when run via `cd web && node --test ...`, but the test
// runs from web/.
const ROOT = process.cwd().endsWith('/web') ? process.cwd() : join(process.cwd(), 'web');
const DISCOVERY_DIR = join(ROOT, 'src/components/Discovery');
const HOOK_FILE = join(ROOT, 'src/hooks/usePgDiscovery.ts');

const COMPONENT_FILES = [
  'DiscoverySection.tsx',
  'DiscoveryBanner.tsx',
  'CycleStatsCard.tsx',
  'ScoreHistoryCard.tsx',
  'WhyRejectedCard.tsx',
  'PromoteTimeline.tsx',
];

function readComp(name: string): string {
  return readFileSync(join(DISCOVERY_DIR, name), 'utf8');
}

test('Test 1: all 6 Discovery component files exist on disk', () => {
  for (const f of COMPONENT_FILES) {
    const p = join(DISCOVERY_DIR, f);
    assert.ok(existsSync(p), `missing component: ${f}`);
  }
});

test('Test 2: usePgDiscovery hook exists', () => {
  assert.ok(existsSync(HOOK_FILE), `missing hook: ${HOOK_FILE}`);
});

test('Test 3: each component exports a named function matching its filename', () => {
  for (const f of COMPONENT_FILES) {
    const name = f.replace(/\.tsx$/, '');
    const src = readComp(f);
    // Match either:  export const X  |  export function X  |  export default function X
    const re = new RegExp(`export\\s+(const|function|default\\s+function)\\s+${name}\\b`);
    assert.ok(
      re.test(src) || src.includes(`export { ${name}`) || src.includes(`export default ${name}`),
      `${f}: no export named ${name} found`,
    );
  }
});

test('Test 4: DiscoverySection renders all 5 cards by importing them', () => {
  const src = readComp('DiscoverySection.tsx');
  const required = [
    'DiscoveryBanner',
    'CycleStatsCard',
    'ScoreHistoryCard',
    'WhyRejectedCard',
    'PromoteTimeline',
  ];
  for (const r of required) {
    assert.ok(src.includes(r), `DiscoverySection.tsx does not reference ${r}`);
  }
});

test('Test 5: DiscoverySection encodes the three scanner states', () => {
  const src = readComp('DiscoverySection.tsx');
  // The pill colors / state-pill resolution must reference all three i18n keys.
  for (const k of [
    'pricegap.discovery.scannerOn',
    'pricegap.discovery.scannerOff',
    'pricegap.discovery.scannerErrored',
  ]) {
    assert.ok(src.includes(k), `DiscoverySection.tsx missing reference to ${k}`);
  }
  // Variant resolution covers errored / disabled / null.
  assert.ok(src.includes("'errored'"), 'errored variant not resolved');
  assert.ok(src.includes("'disabled'"), 'disabled variant not resolved');
});

test('Test 6: CycleStatsCard renders 7 tiles with mono tabular numerics', () => {
  const src = readComp('CycleStatsCard.tsx');
  for (const tid of [
    'tile-last-run',
    'tile-next-run-in',
    'tile-candidates-seen',
    'tile-accepted',
    'tile-rejected',
    'tile-errors',
    'tile-duration',
  ]) {
    assert.ok(src.includes(tid), `CycleStatsCard.tsx missing tile: ${tid}`);
  }
  assert.ok(/font-mono\s+tabular-nums/.test(src), 'mono tabular-nums missing');
  // Renders i18n "Never" for empty last-run.
  assert.ok(src.includes('pricegap.discovery.cycle.never'), '"Never" key not used');
});

test('Test 7: ScoreHistoryCard wires Recharts subcomponents + threshold band', () => {
  const src = readComp('ScoreHistoryCard.tsx');
  for (const r of [
    'ResponsiveContainer',
    'LineChart',
    'Line',
    'XAxis',
    'YAxis',
    'Tooltip',
    'CartesianGrid',
    'ReferenceArea',
  ]) {
    assert.ok(src.includes(r), `ScoreHistoryCard.tsx missing Recharts: ${r}`);
  }
  // Threshold band uses Binance-yellow accent (UI-SPEC §Color).
  assert.ok(src.includes('#f0b90b'), 'threshold band accent color missing');
  // Score line uses positive-green stroke.
  assert.ok(src.includes('#0ecb81'), 'score-line stroke color missing');
  // Empty state copy referenced.
  assert.ok(src.includes('pricegap.discovery.scoreHistory.empty'), 'empty state key missing');
  assert.ok(
    src.includes('pricegap.discovery.scoreHistory.noUniverse'),
    'noUniverse state key missing',
  );
});

test('Test 8: WhyRejectedCard renders reason→count rows + handles empty', () => {
  const src = readComp('WhyRejectedCard.tsx');
  assert.ok(src.includes('pricegap.discovery.whyRejected.empty'), 'empty key missing');
  // Iterates entries and sorts desc.
  assert.ok(src.includes('Object.entries'), 'no entries iteration');
  assert.ok(src.includes('sort'), 'no sort');
  // Count column uses tabular-nums.
  assert.ok(/font-mono\s+tabular-nums/.test(src), 'tabular-nums missing on count');
  // Reason allowlist covers all 8 UI-SPEC reasons.
  for (const r of [
    'insufficient_persistence',
    'stale_bbo',
    'insufficient_depth',
    'denylist',
    'bybit_blackout',
    'symbol_not_listed_long',
    'symbol_not_listed_short',
    'sample_error',
  ]) {
    assert.ok(src.includes(r), `WhyRejectedCard.tsx missing reason: ${r}`);
  }
});

test('Test 9: PromoteTimeline uses solid border (Phase 12 — replaces placeholder)', () => {
  const src = readComp('PromoteTimeline.tsx');
  assert.ok(
    !/border-dashed/.test(src),
    'PromoteTimeline should not use dashed border (replaced placeholder)',
  );
  assert.ok(src.includes('pricegap.discovery.timeline.title'), 'title key missing');
});

test('Test 10: usePgDiscovery hook seeds via REST + subscribes to 3 WS channels', () => {
  const src = readFileSync(HOOK_FILE, 'utf8');
  // REST endpoints (UI-SPEC §"Interaction & Data Contract").
  assert.ok(src.includes('/api/pg/discovery/state'), 'state endpoint missing');
  assert.ok(src.includes('/api/pg/discovery/scores/'), 'scores endpoint missing');
  // WS event types.
  for (const t of ['pg_scan_cycle', 'pg_scan_metrics', 'pg_scan_score']) {
    assert.ok(src.includes(t), `WS event missing: ${t}`);
  }
  // Hook returns the documented surface.
  for (const k of [
    'state',
    'scores',
    'loadScoresFor',
    'errored',
    'enabled',
    'lastRunAt',
  ]) {
    assert.ok(src.includes(k), `hook surface missing: ${k}`);
  }
});

test('Test 11: PriceGap.tsx integrates DiscoverySection between master toggle and candidates', () => {
  const src = readFileSync(join(ROOT, 'src/pages/PriceGap.tsx'), 'utf8');
  assert.ok(src.includes('<DiscoverySection'), 'DiscoverySection not rendered in PriceGap.tsx');
  assert.ok(
    src.includes("from '../components/Discovery/DiscoverySection.tsx'") ||
      src.includes('from "../components/Discovery/DiscoverySection.tsx"'),
    'DiscoverySection import missing',
  );
  // Order check: the import + tag come after the Section 1 comment and before
  // the Section 2 comment.
  const sec1 = src.indexOf('Section 1: Header');
  const tag = src.indexOf('<DiscoverySection');
  const sec2 = src.indexOf('Section 2: Candidates');
  assert.ok(sec1 < tag, 'DiscoverySection rendered before Section 1');
  assert.ok(tag < sec2, 'DiscoverySection rendered after Section 2');
});

test('Test 12: zero new npm dependencies — no third-party imports beyond locked set', () => {
  // The locked deps in web/package.json are: react, react-dom, react-is, recharts.
  // No Discovery component or hook may import anything outside that allowlist.
  const ALLOWED = new Set([
    'react',
    'react-dom',
    'react-is',
    'recharts',
  ]);
  const targets = [
    HOOK_FILE,
    ...COMPONENT_FILES.map((f) => join(DISCOVERY_DIR, f)),
  ];
  const importRe = /from\s+['"]([^'"./]+)(?:\/[^'"]*)?['"]/g;
  for (const f of targets) {
    const src = readFileSync(f, 'utf8');
    let m: RegExpExecArray | null;
    while ((m = importRe.exec(src))) {
      const pkg = m[1]!;
      if (pkg.startsWith('node:')) continue;
      assert.ok(
        ALLOWED.has(pkg),
        `${f} imports forbidden package: ${pkg}`,
      );
    }
  }
});

test('Test 13 (i18n proof): all keys referenced by Discovery components exist in en.ts', () => {
  const enRec = en as Record<string, string>;
  const targets = [
    HOOK_FILE,
    ...COMPONENT_FILES.map((f) => join(DISCOVERY_DIR, f)),
  ];
  const keyRe = /pricegap\.discovery\.[a-zA-Z0-9._]+/g;
  const seen = new Set<string>();
  for (const f of targets) {
    const src = readFileSync(f, 'utf8');
    for (const k of src.match(keyRe) ?? []) {
      // Skip dynamic key prefixes like "pricegap.discovery.whyRejected.reason." (no trailing token)
      if (k.endsWith('.')) continue;
      seen.add(k);
    }
  }
  // Some keys are constructed via template literal (`...reason.${reason}`),
  // so we only assert that every key with a non-dynamic shape exists.
  for (const k of seen) {
    assert.ok(k in enRec, `i18n key referenced but not in en.ts: ${k}`);
  }
});
