## Open Issues (2026-03-23)

### Merge-time exchange-size sanity check
- **Problem**: When a new fill is about to merge into an existing active position, the exchange may already have residual/orphan exposure from an earlier failed trim or late fill. If we merge anyway, we lose the boundary between the old position and the new add-on while exchange state is already dirty.
- **Why this matters**: `consolidate.go` only validates aggregate totals when multiple local positions share the same exchange leg. Once merged, attribution is lost and the engine can carry a contaminated position forward.
- **Implement in caller, not `mergeIntoPosition`**: Keep `mergeIntoPosition` as a DB mutation helper. Perform live exchange reads before calling it.
- **Proposed flow**:
- Read actual long/short exchange sizes with `getExchangePositionSize(...)` immediately before merge.
- Compute expected aggregate sizes:
  `expectedLong = sibling.LongSize + minFill`
  `expectedShort = sibling.ShortSize + minFill`
- Compare expected vs actual using tolerance `max(stepSize, expected*1%)`.
- Only call `mergeIntoPosition(...)` if both legs match within tolerance.
- If mismatch remains, block the merge, log the aggregate mismatch, and quarantine/unwind the new add-on instead of absorbing it into the sibling.
- **Where to wire it**:
- `internal/engine/engine.go` in `executeTradeV2WithPos(...)` right before the existing `mergeIntoPosition(...)` call.
- `internal/engine/engine.go` in the older `executeTrade(...)` path before its merge call as well.
- **Suggested helper**:
- Add `verifyMergeAggregate(existing, symbol, longExch, shortExch, addLong, addShort) error` in `internal/engine/engine.go`.
- **Mismatch behavior**:
- Mark the pending add-on as `exiting`.
- Close only the new balanced fill (`minFill` on both legs).
- Leave the older sibling position untouched.
- Let the consolidator continue recovering any pre-existing residual exposure.
