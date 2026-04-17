#!/usr/bin/env python3
# Backfill two Bybit `GetClosePnL` bugs present in pkg/exchange/bybit/adapter.go
# before v0.32.19:
#
#   Bug 1 (short leg only) — `pricePnL = cumExit - cumEntry` was unsigned for
#   direction, so winning shorts stored ShortClosePnL with the wrong sign
#   (right magnitude). BasisGainLoss = long.PricePnL + short.PricePnL also
#   inherits the flip.
#
#   Bug 2 (either leg) — NetPnL was set to Bybit's `closedPnl`, but closedPnl
#   excludes funding. Every other adapter returns NetPnL funding-inclusive, so
#   reconcile (long.NetPnL + short.NetPnL) silently dropped Bybit's funding
#   term for any position where Bybit was a leg.
#
# Script updates reconciled history entries in place:
#
#   • short_exchange == bybit  ->  flip short_close_pnl sign, recompute
#                                  basis_gain_loss, realized_pnl += short_funding
#   • long_exchange  == bybit  ->  realized_pnl += long_funding
#
# Dry-run by default. Use --apply to persist.
#
# Usage:
#   python3 scripts/backfill_bybit_short_pnl.py
#   python3 scripts/backfill_bybit_short_pnl.py --apply
#
# Requires redis-cli on PATH and REDIS_PASSWORD (or --password).

import argparse
import json
import os
import subprocess
import sys

KEY = "arb:history"
DB = "2"


def redis_cmd(password: str, *args: str):
    res = subprocess.run(
        ["redis-cli", "-n", DB, "-a", password, "--no-auth-warning", *args],
        check=True,
        capture_output=True,
        text=True,
    )
    return res.stdout


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--apply", action="store_true", help="write corrected entries back (default: dry run)")
    ap.add_argument(
        "--password",
        default=os.environ.get("REDIS_PASSWORD", ""),
        help="Redis password (default: $REDIS_PASSWORD)",
    )
    args = ap.parse_args()

    if not args.password:
        print("ERROR: set REDIS_PASSWORD or pass --password", file=sys.stderr)
        return 2

    raw = redis_cmd(args.password, "LRANGE", KEY, "0", "-1")
    lines = [ln for ln in raw.splitlines() if ln.strip()]
    print(f"Scanned {len(lines)} entries in {KEY}")

    fixed_entries = 0
    flipped_short = 0
    adjusted_realized = 0

    for idx, line in enumerate(lines):
        try:
            pos = json.loads(line)
        except json.JSONDecodeError:
            continue

        if pos.get("status") != "closed":
            continue
        if not pos.get("has_reconciled"):
            continue

        long_exch = pos.get("long_exchange")
        short_exch = pos.get("short_exchange")
        if long_exch != "bybit" and short_exch != "bybit":
            continue

        changed = False
        msg_parts = [f"id={pos.get('id')}"]

        # ---- Bug 1: short sign flip (only when Bybit is the short leg) ----
        if short_exch == "bybit":
            old_short = pos.get("short_close_pnl", 0.0)
            unreal = pos.get("short_unrealized_pnl", 0.0)
            # Skip if already correct: flipped entries have short_close_pnl sign
            # matching short_unrealized_pnl (engine's own correct-sign calc).
            # Tolerate tiny magnitudes (sign undefined).
            needs_flip = (
                abs(old_short) >= 1e-6
                and abs(unreal) >= 1e-6
                and (old_short > 0) != (unreal > 0)
            )
            if needs_flip:
                new_short = -old_short
                old_long = pos.get("long_close_pnl", 0.0)
                old_basis = pos.get("basis_gain_loss", 0.0)
                new_basis = old_long + new_short

                msg_parts.append(
                    f"flip short_close_pnl {old_short:+.4f}->{new_short:+.4f}"
                )
                msg_parts.append(
                    f"basis_gain_loss {old_basis:+.4f}->{new_basis:+.4f}"
                )

                pos["short_close_pnl"] = new_short
                pos["basis_gain_loss"] = new_basis
                changed = True
                flipped_short += 1

        # ---- Bug 2: add Bybit leg's funding to realized_pnl ----
        # Applies to any reconciled entry where Bybit is a leg (long or short).
        # We detect "already fixed" by absence of a sentinel marker in the JSON;
        # mark patched entries with `_bybit_funding_backfilled: true`.
        if not pos.get("_bybit_funding_backfilled"):
            bybit_leg_funding = 0.0
            if short_exch == "bybit":
                bybit_leg_funding = pos.get("short_funding", 0.0)
            elif long_exch == "bybit":
                bybit_leg_funding = pos.get("long_funding", 0.0)

            if abs(bybit_leg_funding) >= 1e-9:
                old_realized = pos.get("realized_pnl", 0.0)
                new_realized = old_realized + bybit_leg_funding
                msg_parts.append(
                    f"realized_pnl {old_realized:+.4f}->{new_realized:+.4f} "
                    f"(+{bybit_leg_funding:+.4f} bybit_{'short' if short_exch == 'bybit' else 'long'}_funding)"
                )
                pos["realized_pnl"] = new_realized
                adjusted_realized += 1
                changed = True

            # Mark regardless of whether funding was nonzero — prevents repeated
            # adjustment on re-run and flags the entry as seen by this backfill.
            pos["_bybit_funding_backfilled"] = True
            changed = True

        if changed:
            fixed_entries += 1
            print(f"  [fix] idx={idx} " + " | ".join(msg_parts))

            if args.apply:
                new_json = json.dumps(pos, separators=(",", ":"))
                redis_cmd(args.password, "LSET", KEY, str(idx), new_json)

    verb = "WROTE" if args.apply else "would fix (dry run)"
    print(f"\n{verb}: {fixed_entries} entries "
          f"(short-sign flips: {flipped_short}, realized_pnl adjustments: {adjusted_realized})")
    if not args.apply and fixed_entries > 0:
        print("Re-run with --apply to persist.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
