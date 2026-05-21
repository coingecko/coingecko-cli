package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/coingecko/coingecko-cli/internal/api"
	"github.com/coingecko/coingecko-cli/internal/display"

	"github.com/spf13/cobra"
)

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Get token price by contract address",
	Long: `Fetch token price by contract address. Uses CoinGecko's aggregated price by
default, or DEX price from GeckoTerminal with --onchain.

When --platform and --network are omitted, smart routing automatically detects
the token's network via pool search, then tries the aggregated CoinGecko price
first and falls back to onchain DEX price if unavailable.

Find valid --platform IDs:    https://docs.coingecko.com/reference/asset-platforms-list
Find valid --network IDs:     https://docs.coingecko.com/reference/networks-list

Note: --platform (e.g. "ethereum") and --network (e.g. "eth") are different
identifiers from different API specs — they are not interchangeable.`,
	Example: `  cg contract --address 0x1f98...                        # smart routing
  cg contract --address 0x1f98... --platform ethereum    # explicit CG aggregated
  cg contract --address 0x1f98... --platform ethereum --vs eur
  cg contract --address 0x1f98... --platform ethereum --vs usd,eur,sgd
  cg contract --address 0x1f98... --onchain              # smart routing, onchain only
  cg contract --address 0x1f98... --network eth --onchain
  cg contract --address 0x1f98... --network eth --onchain --vs eur`,
	RunE: runContract,
}

func init() {
	contractCmd.Flags().String("address", "", "Contract address (required)")
	contractCmd.Flags().String("platform", "", "Platform ID for aggregated mode (e.g. ethereum). See https://docs.coingecko.com/reference/asset-platforms-list")
	contractCmd.Flags().String("network", "", "Network ID for onchain mode (e.g. eth). See https://docs.coingecko.com/reference/networks-list")
	contractCmd.Flags().Bool("onchain", false, "Use DEX price from GeckoTerminal")
	contractCmd.Flags().String("vs", "usd", "Target currency (comma-separated for multiple, e.g. usd,eur,sgd)")
	contractCmd.Flags().String("export", "", "Export to CSV file path")
	rootCmd.AddCommand(contractCmd)
}

type resolvedAddress struct {
	network  string // onchain network ID (e.g. "eth")
	platform string // CG asset platform ID (e.g. "ethereum"), may be empty
}

// contractRow holds price data for one currency.
type contractRow struct {
	currency  string
	price     float64
	marketCap float64
	volume    float64
	change    float64
	reserve   float64
}

// resolveAddress searches onchain pools to find which network a contract address
// lives on, then maps the network to its CoinGecko asset platform ID.
// The two API calls run in parallel since they are independent.
func resolveAddress(ctx context.Context, client *api.Client, address string) (*resolvedAddress, error) {
	var (
		pools    *api.OnchainSearchPoolsResponse
		networks *api.OnchainNetworksResponse
		poolsErr, networksErr error
	)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		pools, poolsErr = client.OnchainSearchPools(ctx, address)
	}()
	go func() {
		defer wg.Done()
		networks, networksErr = client.OnchainNetworks(ctx)
	}()
	wg.Wait()

	if poolsErr != nil {
		return nil, fmt.Errorf("searching for address: %w", poolsErr)
	}
	if len(pools.Data) == 0 {
		return nil, fmt.Errorf("no pools found for address %s — specify --platform or --network manually", address)
	}

	// Token relationship IDs follow the format "{network}_{token_address}".
	// Find the token matching the queried address and extract the network prefix.
	addrLower := strings.ToLower(address)
	pool := pools.Data[0]
	var networkID string
	for _, tokenID := range []string{
		pool.Relationships.BaseToken.Data.ID,
		pool.Relationships.QuoteToken.Data.ID,
	} {
		lower := strings.ToLower(tokenID)
		if strings.HasSuffix(lower, "_"+addrLower) {
			networkID = tokenID[:len(tokenID)-len(address)-1]
			break
		}
	}
	if networkID == "" {
		return nil, fmt.Errorf("could not determine network for address %s", address)
	}

	if networksErr != nil {
		return nil, fmt.Errorf("fetching network mappings: %w", networksErr)
	}

	var platformID string
	for _, n := range networks.Data {
		if n.ID == networkID {
			platformID = n.Attributes.CoingeckoAssetPlatformID
			break
		}
	}

	return &resolvedAddress{network: networkID, platform: platformID}, nil
}

func runContract(cmd *cobra.Command, args []string) error {
	address, _ := cmd.Flags().GetString("address")
	platform, _ := cmd.Flags().GetString("platform")
	network, _ := cmd.Flags().GetString("network")
	onchain, _ := cmd.Flags().GetBool("onchain")
	vsRaw, _ := cmd.Flags().GetString("vs")
	currencies := splitTrim(strings.ToLower(vsRaw))
	if len(currencies) == 0 {
		currencies = []string{"usd"}
	}
	exportPath, _ := cmd.Flags().GetString("export")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	if address == "" {
		return fmt.Errorf("--address is required")
	}
	if onchain && platform != "" {
		return fmt.Errorf("--platform and --onchain are mutually exclusive")
	}
	if network != "" && !onchain {
		return fmt.Errorf("--network requires --onchain flag; did you mean --platform?")
	}

	needsResolve := (!onchain && platform == "") || (onchain && network == "")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// vs as comma-separated string for API params.
	vs := strings.Join(currencies, ",")

	if isDryRun(cmd) {
		if needsResolve {
			note := "Smart routing: searches pools to determine network, then fetches /onchain/networks for platform mapping. " +
				"After resolution, the aggregated CG endpoint is tried first; if no data, falls back to onchain."
			return printDryRunFull(cfg, "contract", "resolve",
				"/onchain/search/pools",
				map[string]string{"query": address}, nil, note)
		}
		if onchain {
			params := map[string]string{
				"include_market_cap":           "true",
				"include_24hr_vol":             "true",
				"include_24hr_price_change":    "true",
				"mcap_fdv_fallback":            "true",
				"include_total_reserve_in_usd": "true",
			}
			note := ""
			if vs != "usd" {
				note = fmt.Sprintf("Additional request: GET /exchange_rates (currency conversion from USD to %s)", vs)
			}
			return printDryRunFull(cfg, "contract", "--onchain",
				fmt.Sprintf("/onchain/simple/networks/%s/token_price/%s", url.PathEscape(network), address),
				params, nil, note)
		}
		params := map[string]string{
			"contract_addresses":  address,
			"vs_currencies":       vs,
			"include_market_cap":  "true",
			"include_24hr_vol":    "true",
			"include_24hr_change": "true",
		}
		return printDryRunWithOp(cfg, "contract", "default",
			"/simple/token_price/"+url.PathEscape(platform), params, nil)
	}

	client := newAPIClient(cfg)
	ctx := cmd.Context()

	if needsResolve {
		resolved, err := resolveAddress(ctx, client, address)
		if err != nil {
			return err
		}

		if resolved.platform != "" {
			warnf("Resolved address to network=%s, platform=%s\n", resolved.network, resolved.platform)
		} else {
			warnf("Resolved address to network=%s (no CG platform mapping)\n", resolved.network)
		}

		if onchain {
			network = resolved.network
		} else {
			platform = resolved.platform
			network = resolved.network
		}
	}

	var rows []contractRow

	// CG aggregated: API natively supports multiple vs_currencies.
	if !onchain && platform != "" {
		resp, err := client.SimpleTokenPrice(ctx, platform, []string{address}, vs)
		if err != nil {
			return err
		}
		if data, ok := resp[address]; ok {
			for _, cur := range currencies {
				rows = append(rows, contractRow{
					currency:  cur,
					price:     data[cur],
					marketCap: data[cur+"_market_cap"],
					volume:    data[cur+"_24h_vol"],
					change:    data[cur+"_24h_change"],
				})
			}
		} else if network != "" {
			warnf("No aggregated price found; falling back to onchain (network=%s)\n", network)
			onchain = true
		} else {
			return fmt.Errorf("no data returned for address %s", address)
		}
	}

	if !onchain && platform == "" && network != "" {
		warnf("No CG platform mapping; using onchain (network=%s)\n", network)
		onchain = true
	}

	// Onchain: USD only, convert to each requested currency via /exchange_rates.
	if onchain {
		resp, err := client.OnchainSimpleTokenPrice(ctx, network, []string{address})
		if err != nil {
			return err
		}

		attrs := resp.Data.Attributes
		priceStr, ok := attrs.TokenPrices[address]
		if !ok {
			return fmt.Errorf("no data returned for address %s", address)
		}

		priceUSD, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return fmt.Errorf("parsing price: %w", err)
		}

		var mcapUSD, volUSD, changeUSD, reserveUSD float64
		if v, ok := attrs.MarketCapUSD[address]; ok {
			mcapUSD, _ = strconv.ParseFloat(v, 64)
		}
		if v, ok := attrs.H24VolumeUSD[address]; ok {
			volUSD, _ = strconv.ParseFloat(v, 64)
		}
		if v, ok := attrs.H24PriceChangePct[address]; ok {
			changeUSD, _ = strconv.ParseFloat(v, 64)
		}
		if v, ok := attrs.TotalReserveInUSD[address]; ok {
			reserveUSD, _ = strconv.ParseFloat(v, 64)
		}

		// Single /exchange_rates call covers all currencies.
		needsConversion := len(currencies) > 1 || currencies[0] != "usd"
		var rates *api.ExchangeRatesResponse
		if needsConversion {
			rates, err = client.ExchangeRates(ctx)
			if err != nil {
				return fmt.Errorf("fetching exchange rates: %w", err)
			}
		}

		for _, cur := range currencies {
			row := contractRow{
				currency:  cur,
				price:     priceUSD,
				marketCap: mcapUSD,
				volume:    volUSD,
				change:    changeUSD,
				reserve:   reserveUSD,
			}
			if cur != "usd" {
				usdRate, usdOK := rates.Rates["usd"]
				targetRate, targetOK := rates.Rates[cur]
				if !usdOK || !targetOK {
					return fmt.Errorf("unsupported currency %q", cur)
				}
				factor := targetRate.Value / usdRate.Value
				row.price *= factor
				row.marketCap *= factor
				row.volume *= factor
				row.reserve *= factor
				// 24h change % stays the same
			}
			rows = append(rows, row)
		}
	}

	if jsonOut {
		if len(currencies) == 1 {
			// Single currency: flat output (backward compatible).
			r := rows[0]
			data := map[string]interface{}{
				"price":      r.price,
				"market_cap": r.marketCap,
				"volume_24h": r.volume,
				"change_24h": r.change,
			}
			if onchain {
				data["total_reserve"] = r.reserve
			}
			return printJSONRaw(map[string]interface{}{address: data})
		}
		// Multiple currencies: nested by currency.
		currencyData := make(map[string]interface{}, len(rows))
		for _, r := range rows {
			data := map[string]interface{}{
				"price":      r.price,
				"market_cap": r.marketCap,
				"volume_24h": r.volume,
				"change_24h": r.change,
			}
			if onchain {
				data["total_reserve"] = r.reserve
			}
			currencyData[r.currency] = data
		}
		return printJSONRaw(map[string]interface{}{address: currencyData})
	}

	headers := []string{"Address", "Currency", "Price", "Market Cap", "24h Volume", "24h Change"}
	if onchain {
		headers = append(headers, "Reserve")
	}
	// Single currency: omit Currency column for cleaner output.
	if len(currencies) == 1 {
		headers = []string{"Address", "Price", "Market Cap", "24h Volume", "24h Change"}
		if onchain {
			headers = append(headers, "Reserve")
		}
	}

	var tableRows [][]string
	var csvRows [][]string
	for _, r := range rows {
		var tableRow, csvRow []string
		if len(currencies) == 1 {
			tableRow = []string{
				display.SanitizeCell(address),
				display.FormatPrice(r.price, r.currency),
				display.FormatLargeNumber(r.marketCap, r.currency),
				display.FormatLargeNumber(r.volume, r.currency),
				display.ColorPercent(r.change),
			}
			csvRow = []string{
				display.SanitizeCell(address),
				fmt.Sprintf("%.8f", r.price),
				fmt.Sprintf("%.2f", r.marketCap),
				fmt.Sprintf("%.2f", r.volume),
				fmt.Sprintf("%.2f", r.change),
			}
		} else {
			tableRow = []string{
				display.SanitizeCell(address),
				strings.ToUpper(r.currency),
				display.FormatPrice(r.price, r.currency),
				display.FormatLargeNumber(r.marketCap, r.currency),
				display.FormatLargeNumber(r.volume, r.currency),
				display.ColorPercent(r.change),
			}
			csvRow = []string{
				display.SanitizeCell(address),
				strings.ToUpper(r.currency),
				fmt.Sprintf("%.8f", r.price),
				fmt.Sprintf("%.2f", r.marketCap),
				fmt.Sprintf("%.2f", r.volume),
				fmt.Sprintf("%.2f", r.change),
			}
		}
		if onchain {
			tableRow = append(tableRow, display.FormatLargeNumber(r.reserve, r.currency))
			csvRow = append(csvRow, fmt.Sprintf("%.2f", r.reserve))
		}
		tableRows = append(tableRows, tableRow)
		csvRows = append(csvRows, csvRow)
	}
	display.PrintTable(headers, tableRows)

	if exportPath != "" {
		if err := exportCSV(exportPath, headers, csvRows); err != nil {
			return err
		}
	}

	return nil
}
