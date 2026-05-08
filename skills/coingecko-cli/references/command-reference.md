# CoinGecko CLI Command Reference

## Command Resolution

Prefer the shortest available executable:

```sh
command -v cg || command -v coingecko-cli
```

When inside the source checkout, `make build` creates `./cg`. Use that binary if no installed command exists or if testing local changes.

## Auth

- `cg auth`: save API key and tier interactively.
- `CG_API_KEY=... CG_API_TIER=demo cg auth`: save auth non-interactively.
- `cg status`: inspect configured tier.
- Tiers: `demo` for free/public API, `paid` for pro API endpoints.
- Config file: `~/.config/coingecko-cli/config.yaml`.

Avoid passing secrets through flags unless the user explicitly asks. Environment variables are safer than command flags because flags can appear in shell history and process listings.

## Global Behavior

- `-o json` / `--output json`: emit machine-readable stdout for data commands.
- `--dry-run`: print the planned API/WebSocket request without executing it.
- Stdout is data only; diagnostics and warnings go to stderr.
- `CG_NO_UPDATE_CHECK=1` disables update checks for CI or deterministic scripting.
- `cg commands`: emit the live command catalog with flags, examples, output formats, paid-only markers, API endpoints, OAS operation IDs, and WebSocket metadata.

## Data Commands

### `price`

Current prices by CoinGecko ID or ticker symbol.

```sh
cg price --ids bitcoin,ethereum -o json
cg price --symbols btc,eth --vs eur -o json
```

Flags: `--ids`, `--symbols`, `--vs` (default `usd`).
Endpoint: `/simple/price`.

### `search`

Find CoinGecko coin IDs before precise lookups.

```sh
cg search solana -o json
cg search dog --limit 5 -o json
```

Flag: `--limit` (default `10`).
Endpoint: `/search`.

### `markets`

Rankings, market caps, volumes, category filters, and CSV exports.

```sh
cg markets --total 100 -o json
cg markets --total 500 --vs eur --export top500.csv
cg markets --category artificial-intelligence --total 250 -o json
```

Flags: `--total`, `--vs`, `--order`, `--category`, `--export`.
Order enum: `market_cap_desc`, `market_cap_asc`, `volume_asc`, `volume_desc`, `id_asc`, `id_desc`.
Endpoint: `/coins/markets`.
Pagination: the CLI fetches 250 coins per page and trims to `--total`.

### `trending`

Trending coins, NFTs, and categories.

```sh
cg trending -o json
cg trending --show-max coins,nfts,categories -o json
```

Flag: `--show-max` for expanded paid-plan results.
Endpoint: `/search/trending`.

### `history`

Historical snapshots, price series, ranges, OHLC data, and CSV exports.

```sh
cg history bitcoin --date 2024-01-15 -o json
cg history bitcoin --days 30 --export btc_30d.csv
cg history bitcoin --days 7 --ohlc -o json
cg history solana --from 2024-01-01 --to 2024-03-01 -o json
cg history solana --from 2024-01-01 --to 2024-03-01 --interval daily --export sol_q1.csv
```

Modes are mutually exclusive: `--date`, `--days`, or `--from` plus `--to`.
Flags: `--vs`, `--interval daily|hourly`, `--ohlc`, `--export`.
OHLC `--days` enum: `1`, `7`, `14`, `30`, `90`, `180`, `365`, `max`.
Paid-only: `--ohlc` with date ranges or explicit interval.
Endpoints: `/coins/{id}/history`, `/coins/{id}/market_chart`, `/coins/{id}/market_chart/range`, `/coins/{id}/ohlc`, `/coins/{id}/ohlc/range`.

### `top-gainers-losers`

Top gaining or losing coins. Paid plan only.

```sh
cg top-gainers-losers -o json
cg top-gainers-losers --losers --duration 7d -o json
cg top-gainers-losers --top-coins 300 --export gainers.csv
```

Flags: `--vs`, `--duration`, `--top-coins`, `--losers`, `--price-change-percentage`, `--export`.
Duration enum: `1h`, `24h`, `7d`, `14d`, `30d`, `60d`, `1y`.
Top-coins enum: `300`, `500`, `1000`, `all`.
Endpoint: `/coins/top_gainers_losers`.

### `watch`

Live WebSocket price updates. Paid Analyst plan or above; USD only.

```sh
cg watch --ids bitcoin,ethereum -o json
cg watch --symbols btc,eth --dry-run
```

Flags: `--ids`, `--symbols`.
Transport: `wss://stream.coingecko.com/v1`.
JSON output is NDJSON. Terminate after collecting the requested data unless the user asks for a persistent stream.

## Interactive Commands

Use only when the user asks for a terminal UI.

```sh
cg tui markets
cg tui markets --vs eur --category layer-1
cg tui trending
```

Subcommands: `tui markets`, `tui trending`.

## Maintenance

- `cg version -o json`: show version/build information.
- `cg update`: upgrade the CLI; run only on explicit user request.
- `cg commands`: emit the agent-friendly command catalog.
