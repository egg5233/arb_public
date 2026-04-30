// Phase 12 Plan 04 — PromoteTimeline static-contract tests.
//
// CLAUDE.local.md npm lockdown forbids vitest / @testing-library/react.
// Node's --experimental-strip-types cannot process .tsx (JSX) files, so
// this test file is .ts and uses filesystem + regex contract checks
// instead of mounting React. The visual checkpoint (Plan 04 Task 3)
// covers behavioral correctness end-to-end.
//
// Run with: node --test --experimental-strip-types
//   src/components/Discovery/__tests__/PromoteTimeline.test.ts
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const srcPath = path.resolve(__dirname, '../PromoteTimeline.tsx');
const src = readFileSync(srcPath, 'utf8');

test('PromoteTimeline subscribes to usePgDiscovery hook', () => {
  assert.match(src, /usePgDiscovery\(\)/);
});

test('PromoteTimeline reads promoteEvents/promoteEventsLoading/promoteEventsError', () => {
  assert.match(src, /promoteEvents/);
  assert.match(src, /promoteEventsLoading/);
  assert.match(src, /promoteEventsError/);
});

test('PromoteTimeline uses inherited card chrome (Phase 11 panelCls token)', () => {
  assert.match(src, /bg-gray-900 border border-gray-800 rounded-lg p-4/);
});

test('PromoteTimeline list container is aria-live polite + role=list', () => {
  assert.match(src, /aria-live="polite"/);
  assert.match(src, /role="list"/);
});

test('PromoteTimeline emits sr-only announce string for color-not-only signal', () => {
  assert.match(src, /sr-only/);
  assert.match(src, /timeline\.sr\.promote/);
  assert.match(src, /timeline\.sr\.demote/);
});

test('PromoteTimeline caps visible at 50 default with HARD_CAP 1000', () => {
  assert.match(src, /MAX_VISIBLE_DEFAULT = 50/);
  assert.match(src, /HARD_CAP = 1000/);
});

test('PromoteTimeline renders wsDisconnected badge', () => {
  assert.match(src, /timeline\.wsDisconnected/);
});

test('PromoteTimeline uses Asia/Taipei timezone for timestamps', () => {
  assert.match(src, /Asia\/Taipei/);
});

test('PromoteTimeline uses #0ecb81 promote accent + #f6465d demote accent', () => {
  assert.match(src, /#0ecb81/);
  assert.match(src, /#f6465d/);
});

test('PromoteTimeline references all 5 state-rendering i18n keys', () => {
  for (const k of [
    'timeline.empty',
    'timeline.loading',
    'timeline.seedError',
    'timeline.wsDisconnected',
    'timeline.loadMore',
  ]) {
    assert.ok(src.includes(k), `missing i18n key reference: ${k}`);
  }
});

test('PromoteTimeline does not use border-dashed (phased-out placeholder style)', () => {
  assert.ok(!/border-dashed/.test(src), 'border-dashed must not appear (was placeholder-only)');
});

test('PromoteTimeline interpolates {n} into loadMore label client-side', () => {
  // Locale t() has no built-in interpolation; PromoteTimeline must do .replace('{n}', ...)
  assert.match(src, /'\{n\}'/);
});
