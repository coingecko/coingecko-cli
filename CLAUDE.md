# CLAUDE.md

## Project Overview

`cg` is a CoinGecko CLI tool written in Go. It provides both standard CLI commands and an interactive TUI for browsing cryptocurrency market data from the CoinGecko API.

## Build & Test

```sh
make build       # Build binary → ./cg
make test        # Run tests with -race
make lint        # Run golangci-lint
make clean       # Remove binary
```

Or directly:

```sh
go build -o cg .
go test -race ./...
```

## Project Structure

```
coingecko-cli/
├── main.go                        # Entry point
├── cmd/                           # Cobra commands (auth, status, price, markets, search, trending, history, top_gainers_losers, contract, watch, tui, version)
├── internal/
│   ├── api/
│   │   ├── client.go              # HTTP client, auth, error handling
│   │   ├── client_test.go         # httptest-based tests
│   │   ├── coins.go               # API endpoint methods + FetchAllMarkets pagination helper
│   │   ├── coins_test.go          # FetchAllMarkets tests
│   │   └── types.go               # JSON response structs
│   ├── config/
│   │   ├── config.go              # Viper-based config (API key, tier)
│   │   └── config_test.go
│   ├── display/
│   │   ├── banner.go              # ASCII logo, welcome box, brand banner
│   │   ├── table.go               # Table rendering
│   │   ├── format.go              # Price/number formatting
│   │   └── color.go               # ANSI color (NO_COLOR/TTY aware)
│   ├── export/
│   │   └── csv.go                 # CSV file export
│   ├── ws/
│   │   ├── client.go              # WebSocket client (ActionCable protocol, reconnect, state machine)
│   │   └── client_test.go         # WebSocket client tests (httptest + gorilla/websocket upgrader)
│   └── tui/
│       ├── styles.go              # Shared lipgloss styles, brand colors, frame/layout helpers
│       ├── markets.go             # Markets TUI model
│       ├── trending.go            # Trending TUI model
│       ├── detail.go              # Coin detail view
│       └── chart.go               # Braille price chart (ntcharts)
├── Makefile
├── .goreleaser.yml
├── install.sh
└── .github/workflows/             # CI (lint, test, build) and Release (goreleaser)
```

## Conventions

- **Go version**: 1.26 (pinned in go.mod and CI)
- **Binary name**: `cg`
- **Module path**: `github.com/coingecko/coingecko-cli`
- **Test framework**: `testify/assert` + `net/http/httptest`
- **API client tests**: use `httptest.NewServer` with `SetBaseURL`, never hit real API
- **Config location**: `~/.config/coingecko-cli/config.yaml` (dir `0700`, file `0600`)
- **Auth tiers**: demo, paid — demo uses `x-cg-demo-api-key` header and `api.coingecko.com`, paid uses `x-cg-pro-api-key` and `pro-api.coingecko.com`
- **Error model**: `ErrInvalidAPIKey` (401), `ErrPlanRestricted` (403), `ErrRateLimited` (429) — all user-friendly messages
- **Paid-only endpoints**: use `requirePaid()` pre-flight check before API call
- **Output modes**: global `-o json` / `--output json` flag on all data commands; JSON mode emits raw API structs to stdout
- **Stream contract**: stdout = data only, stderr = diagnostics/warnings (use `warnf()` helper in `cmd/output.go`)
- **Color output**: respects `NO_COLOR` env var and TTY detection — never hardcode ANSI in output that may be piped
- **Formatting**: all code must be `gofmt`-clean
- **Commits**: conventional commit style (`feat:`, `fix:`, `chore:`)
- **TUI framework**: Bubble Tea (bubbletea) + Lip Gloss (lipgloss) + Bubbles + ntcharts
- **Interactive prompts**: `huh` (Charm ecosystem) — API key input uses password echo mode
- **Auth key input priority**: env var `CG_API_KEY` > `--key` flag > interactive prompt — env vars are preferred to avoid shell history/process listing exposure

## API Reference

- **OAS specs**: https://github.com/coingecko/coingecko-api-oas (`coingecko-demo.json`, `coingecko-pro.json`)
- **API docs**: https://docs.coingecko.com
- **Rate limiting**: 429 responses may include `Retry-After: <seconds>` header and/or `x-ratelimit-reset: <timestamp>` header (format `2006-01-02 15:04:05 -0700`). In practice, CoinGecko sends `x-ratelimit-reset` but not always `Retry-After`. The CLI prefers `Retry-After`, then `x-ratelimit-reset`, then local exponential backoff with jitter
- **Structured errors**: `-o json` mode emits `{"error":"rate_limited","message":"...","retry_after":54}` to stderr
- **Dry run**: `--dry-run` on any data command outputs the API request as JSON without executing
- **Command catalog**: `cg commands` (hidden) outputs machine-readable JSON with all commands, flags, OAS operation IDs, and enums

### CLI → OAS Endpoint Mapping

| CLI Command | API Endpoint | OAS Operation ID |
|-------------|-------------|------------------|
| `cg price` | `/simple/price` | `simple-price` |
| `cg markets` | `/coins/markets` | `coins-markets` |
| `cg search` | `/search` | `search-data` |
| `cg trending` | `/search/trending` | `trending-search` |
| `cg history --date` | `/coins/{id}/history` | `coins-id-history` |
| `cg history --days` | `/coins/{id}/market_chart` | `coins-id-market-chart` |
| `cg history --days --interval hourly` | `/coins/{id}/market_chart/range` (batched) | `coins-id-market-chart-range` |
| `cg history --days --ohlc` | `/coins/{id}/ohlc` | `coins-id-ohlc` |
| `cg history --from/--to` | `/coins/{id}/market_chart/range` | `coins-id-market-chart-range` |
| `cg history --from/--to --interval hourly` | `/coins/{id}/market_chart/range` (batched) | `coins-id-market-chart-range` |
| `cg history --from/--to --ohlc` | `/coins/{id}/ohlc/range` (batched for large ranges) | `coins-id-ohlc-range` |
| `cg top-gainers-losers` | `/coins/top_gainers_losers` | `coins-top-gainers-losers` |
| `cg contract` (smart routing) | `/onchain/search/pools` + `/onchain/networks` → `/simple/token_price/{platform}` (CG first, onchain fallback) | `search-pools`, `simple-token-price`, `onchain-simple-price` |
| `cg contract --platform` | `/simple/token_price/{platform}` | `simple-token-price` |
| `cg contract --onchain` | `/onchain/simple/networks/{network}/token_price/{addresses}` | `onchain-simple-price` |
| `cg watch` | `wss://stream.coingecko.com/v1` (WebSocket) | — |

## Distribution

- **Homebrew**: tap repo at `coingecko/homebrew-coingecko-cli` — goreleaser pushes formula on each tagged release
- **Goreleaser**: `.goreleaser.yml` uses `brews` to generate Homebrew formula for CLI binary distribution
- **Release workflow**: `.github/workflows/release.yml` triggers on `v*` tags, requires `HOMEBREW_TAP_TOKEN` repo secret for tap repo write access
- **Tagging**: always tag from `main` after pulling latest — `git tag vX.Y.Z && git push origin vX.Y.Z`
- **Install script**: `install.sh` downloads the latest release binary from GitHub Releases
- **Go install**: `go install github.com/coingecko/coingecko-cli@latest` — produces a binary named `coingecko-cli` (module-path basename). Users alias or symlink it to `cg`; the `cg update --method go` flow prints this heads-up before running install.
- **npm**: `npm install -g @coingecko/cg` — umbrella package in `npm/cg/` plus 6 platform sub-packages (`npm/cg-<os>-<arch>/`) published via OIDC Trusted Publishing from `release.yml`. Each package needs its trusted publisher configured on npmjs.com (Repository: `coingecko/coingecko-cli`, Workflow: `release.yml`) before its first publish. Publish flow lives in `scripts/npm-publish.sh`.
- **`cg update` install methods**: auto-detects `homebrew`, `npm`, `go`, or `script` from the executable path. npm detection (`node_modules/@coingecko/cg-*`) runs before homebrew because Homebrew-installed Node puts npm globals under `/opt/homebrew/lib/node_modules/`.

## Key Design Decisions

- CoinGecko `/coins/{id}/market_chart/range` expects UNIX timestamps in seconds — CLI accepts `YYYY-MM-DD` and converts in the command layer
- CoinGecko `/coins/{id}/history` uses `DD-MM-YYYY` date format — CLI accepts `YYYY-MM-DD` and converts
- Symbol resolution: `cg price --symbols` uses `/simple/price?symbols=` directly (API accepts symbols natively). `cg watch --symbols` uses `/search` to resolve symbols to coin IDs (WebSocket requires coin IDs), picking exact case-insensitive match with highest market_cap_rank
- TUI detail view fetches coin detail + OHLC concurrently via `tea.Batch`
- `RateLimitError` typed error carries `RetryAfter` seconds, satisfies `errors.Is(err, ErrRateLimited)` via custom `Is()` method
- API text (coin names, symbols) sanitized via `display.SanitizeCell` to strip terminal escape injection
- **History batching**: large date ranges are auto-batched into sequential API requests with dedup at chunk boundaries. Batching logic lives in `cmd/history.go` (`fetchMarketChartBatched`, `fetchOHLCRangeBatched`, `dedupTimeseries`)
- **Hourly interval strategy**: `--interval hourly` routes through `/market_chart/range` with no `interval` param, relying on CoinGecko auto-granularity (2–90 day ranges → hourly). Chunks are ≤90 days. Requests for 1 day are padded to 2 days then trimmed
- **Daily interval on /range**: uses auto-granularity (>90 day ranges → daily). Short ranges (<91 days) are padded to 91 days then trimmed to the original range
- **OHLC range batching**: daily chunks ≤170 days (API limit 180), hourly chunks ≤30 days (API limit 31). `--ohlc --interval` requires paid plans
- **`--interval` values**: only `daily` and `hourly` are accepted; `5m` is not supported (Enterprise-only)
- **Hourly data availability**: CoinGecko hourly data is only available from 2018-01-30 onwards; 5-minute data from 2018-02-09 onwards. The CLI validates this client-side and rejects requests with `--interval hourly` before the cutoff date
- **WebSocket streaming**: `cg watch` uses CoinGecko's ActionCable WebSocket API (`CGSimplePrice` channel) for real-time price updates (~10s). Paid-only, USD prices only. API key passed via `x_cg_pro_api_key` query param. Coin IDs are validated via `/simple/price` before connecting; invalid IDs are skipped with a warning
- **WebSocket reconnect**: automatic reconnect with exponential backoff (1s→30s cap + jitter). `Close()` sets an atomic `closing` flag that suppresses reconnect. Single `readLoop` goroutine owns the connection lifecycle
- **WebSocket test seams**: `cmd/client_factory.go` exposes `Streamer` interface + `newStreamer` factory for injecting test doubles in command tests. WS protocol tests use `httptest` + `gorilla/websocket.Upgrader`
- **Command test seams**: `cmd/client_factory.go` exposes injectable `newAPIClient`, `loadConfig`, and `newStreamer` vars so command integration tests can swap in httptest servers and test configs without touching real API or config files
- **Pagination helper**: `FetchAllMarkets` in `internal/api/coins.go` handles multi-page fetching (250/page) with trim-to-total, used by both `cg markets` and `cg tui markets`
- **TUI trending tier awareness**: demo gets 15 coins, paid gets 30 via `show_max=coins` API param
- **Onchain currency conversion**: `--onchain` returns USD only. For `--vs` non-USD, the CLI fetches `/exchange_rates` and multiplies price/mcap/volume by `targetRate/usdRate`. 24h change % is unchanged. No caching — one extra API call per non-USD onchain request
- **Platform/network discoverability**: `--platform` IDs come from `/asset_platforms` ([docs](https://docs.coingecko.com/reference/asset-platforms-list)), `--network` IDs from `/onchain/networks` ([docs](https://docs.coingecko.com/reference/networks-list)). These are different namespaces but auto-translated via the `coingecko_asset_platform_id` field in `/onchain/networks`
- **Contract smart routing**: when `--platform`/`--network` are omitted, `resolveAddress()` fires `/onchain/search/pools` and `/onchain/networks` in parallel, extracts the network from the token relationship ID prefix (`{network}_{address}`), maps to CG platform via `coingecko_asset_platform_id`, then tries CG aggregated price first with onchain fallback. Address lookups are case-insensitive (normalized to lowercase)
- **Onchain FDV fallback**: `OnchainSimpleTokenPrice` always sends `mcap_fdv_fallback=true` — the API returns FDV in `market_cap_usd` when market cap is unavailable
- **Onchain reserve/liquidity**: `OnchainSimpleTokenPrice` always sends `include_total_reserve_in_usd=true` — the Reserve column appears only in onchain mode output (table/JSON/CSV)
