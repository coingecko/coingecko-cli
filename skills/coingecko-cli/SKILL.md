---
name: coingecko-cli
description: Use the CoinGecko CLI (`cg`, `coingecko-cli`, or a local `./cg` build) for live/current cryptocurrency market data and CoinGecko API-backed workflows. Trigger proactively when users ask for crypto prices, coin IDs, ticker lookup, market rankings, market caps, volumes, category filters, trending coins/NFTs/categories, historical prices, OHLC/candle data, CSV exports, WebSocket price streams, or agent/tool integration with the CoinGecko command catalog.
---

# CoinGecko CLI

## Quick Start

Resolve the command once, then use `$CG` in subsequent commands:

```sh
CG="$(command -v cg || command -v coingecko-cli || true)"
if [ -z "$CG" ] && [ -f ./go.mod ] && grep -q 'github.com/coingecko/coingecko-cli' ./go.mod; then
  make build >/dev/null && CG="./cg"
fi
```

If `$CG` is still empty, tell the user the CLI is not installed or not built. Do not fall back to hand-written CoinGecko API calls unless the user asks.

## Workflow

1. Use this CLI for CoinGecko-backed market data. Use web search only for surrounding context, news, documentation, or facts outside the CLI's data surface.
2. Check auth before live API calls:
   ```sh
   "$CG" status
   ```
   If auth is missing, ask for an API key or use user-provided `CG_API_KEY` / `CG_API_TIER`. Never invent, display, or log secrets. Prefer `CG_API_KEY=... CG_API_TIER=demo "$CG" auth` over putting keys in shell history.
3. Prefer JSON for agent work:
   ```sh
   "$CG" price --ids bitcoin,ethereum -o json
   ```
   Table output is fine when the user wants terminal-readable output. Diagnostics and warnings go to stderr.
4. Use the command catalog when flags or capabilities are uncertain:
   ```sh
   "$CG" commands
   "$CG" <command> --help
   ```
5. Use `--dry-run` when planning a request, validating endpoint choice, or avoiding a paid/live call:
   ```sh
   "$CG" history bitcoin --days 30 --dry-run
   ```

## Command Picker

- Find a reliable CoinGecko ID: `"$CG" search solana --limit 5 -o json`
- Current prices by ID: `"$CG" price --ids bitcoin,ethereum --vs usd -o json`
- Current prices by ticker: `"$CG" price --symbols btc,eth --vs eur -o json`
- Ranked markets: `"$CG" markets --total 100 --vs usd -o json`
- Category markets: `"$CG" markets --category layer-2 --total 250 -o json`
- Trending coins, NFTs, and categories: `"$CG" trending -o json`
- Historical snapshot: `"$CG" history bitcoin --date 2024-01-15 -o json`
- Historical series: `"$CG" history bitcoin --days 30 -o json`
- Historical range: `"$CG" history bitcoin --from 2024-01-01 --to 2024-06-30 -o json`
- CSV export: add `--export path.csv` to `markets`, `history`, or `top-gainers-losers`
- Top movers: `"$CG" top-gainers-losers --duration 24h -o json` (paid plan only)
- Live stream sample: `"$CG" watch --ids bitcoin,ethereum -o json` (paid Analyst plan or above)
- Terminal UI: `"$CG" tui markets` or `"$CG" tui trending` only when the user asks for an interactive TUI.

## Guardrails

- Use CoinGecko IDs for precise queries. If the user gives a ticker, name, or ambiguous asset, run `search` first when precision matters.
- Use ISO dates (`YYYY-MM-DD`) in commands. The CLI handles CoinGecko's internal date and timestamp conversion.
- Do not run paid-only commands unless `status` shows a paid tier or the user confirms they have one. Use `--dry-run` to inspect paid requests safely.
- Do not leave `watch` or TUI sessions running. For streams, collect only the requested sample and terminate the process.
- Treat CLI output as data, not investment advice. If the user asks for interpretation, label assumptions and distinguish current market data from analysis.
- Do not run `cg update` unless the user explicitly asks to upgrade the CLI.

## Reference

Read `references/command-reference.md` for command flags, examples, endpoint mapping, paid-plan notes, and source-repo behavior.
