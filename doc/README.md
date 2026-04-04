# Docs Index

This directory contains two different kinds of exchange documentation:

- repo-curated summaries and audits in the `doc/` root
- raw or lightly cleaned vendor docs under `doc/{exchange}/`

If you are changing adapter code, start with the exchange folder README and then open the specific vendor file for the endpoint family you need.

## Start Here

- System design and trading-specific notes: [ARCHITECTURE.md](/var/solana/data/arb/ARCHITECTURE.md)
- Spot-futures risk notes: [DESIGN_SPOT_FUTURES_RISK.md](/var/solana/data/arb/doc/DESIGN_SPOT_FUTURES_RISK.md)
- Shared exchange references:
  - [base-urls.md](/var/solana/data/arb/doc/references/base-urls.md)
  - [authentication.md](/var/solana/data/arb/doc/references/authentication.md)

## Exchange Folders

- Binance: [README.md](/var/solana/data/arb/doc/binance/README.md)
- BingX: [README.md](/var/solana/data/arb/doc/bingx/README.md)
- Bitget: [README.md](/var/solana/data/arb/doc/bitget/README.md)
- Bybit: [README.md](/var/solana/data/arb/doc/bybit/README.md)
- Gate.io: [README.md](/var/solana/data/arb/doc/gate/README.md)
- OKX: [README.md](/var/solana/data/arb/doc/okx/README.md)

## Root Summaries

These are repo-authored notes and are usually faster to skim than the vendor docs:

- Binance:
  - [EXCHANGEAPI_BINANCE.md](/var/solana/data/arb/doc/EXCHANGEAPI_BINANCE.md)
  - [EXCHANGEAPI_BINANCE_MARGIN.md](/var/solana/data/arb/doc/EXCHANGEAPI_BINANCE_MARGIN.md)
  - [AUDIT_BINANCE.md](/var/solana/data/arb/doc/AUDIT_BINANCE.md)
  - [SPEC_BINANCE_SPOT_MARGIN.md](/var/solana/data/arb/doc/SPEC_BINANCE_SPOT_MARGIN.md)
- BingX:
  - [EXCHANGEAPI_BINGX.md](/var/solana/data/arb/doc/EXCHANGEAPI_BINGX.md)
  - [AUDIT_BINGX.md](/var/solana/data/arb/doc/AUDIT_BINGX.md)
- Bitget:
  - [EXCHANGEAPI_BITGET.md](/var/solana/data/arb/doc/EXCHANGEAPI_BITGET.md)
  - [EXCHANGEAPI_BITGET_MARGIN.md](/var/solana/data/arb/doc/EXCHANGEAPI_BITGET_MARGIN.md)
  - [AUDIT_BITGET.md](/var/solana/data/arb/doc/AUDIT_BITGET.md)
- Bybit:
  - [EXCHANGEAPI_BYBIT.md](/var/solana/data/arb/doc/EXCHANGEAPI_BYBIT.md)
  - [EXCHANGEAPI_BYBIT_MARGIN.md](/var/solana/data/arb/doc/EXCHANGEAPI_BYBIT_MARGIN.md)
  - [AUDIT_BYBIT.md](/var/solana/data/arb/doc/AUDIT_BYBIT.md)
- Gate.io:
  - [EXCHANGEAPI_GATEIO.md](/var/solana/data/arb/doc/EXCHANGEAPI_GATEIO.md)
  - [EXCHANGEAPI_GATEIO_MARGIN.md](/var/solana/data/arb/doc/EXCHANGEAPI_GATEIO_MARGIN.md)
  - [EXCHANGEAPI_GATEIO_UNIFIED.md](/var/solana/data/arb/doc/EXCHANGEAPI_GATEIO_UNIFIED.md)
  - [AUDIT_GATEIO.md](/var/solana/data/arb/doc/AUDIT_GATEIO.md)
- OKX:
  - [EXCHANGEAPI_OKX.md](/var/solana/data/arb/doc/EXCHANGEAPI_OKX.md)
  - [EXCHANGEAPI_OKX_MARGIN.md](/var/solana/data/arb/doc/EXCHANGEAPI_OKX_MARGIN.md)
  - [AUDIT_OKX.md](/var/solana/data/arb/doc/AUDIT_OKX.md)

## Working Pattern

For exchange changes, this order is usually fastest:

1. Open the exchange folder README.
2. Read the relevant root summary or audit if one exists.
3. Open the specific vendor doc for the endpoint family you are touching.
4. Verify the adapter implementation in `pkg/exchange/{exchange}/`.

The vendor docs in `doc/{exchange}/` are still source material. Some have been cleaned for readability, but they should not be treated as fully normalized internal docs.
