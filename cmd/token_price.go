package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/coingecko/coingecko-cli/internal/display"

	"github.com/spf13/cobra"
)

var tokenPriceCmd = &cobra.Command{
	Use:   "token-price",
	Short: "Get current price for tokens by contract address",
	Long:  "Fetch current prices by token contract addresses on a specific platform. Use --address for one or more contract addresses and --platform for the chain (e.g. ethereum, base, arbitrum-one).",
	Example: `  cg token-price --address 0x1f9840a85d5af5bf1d1762f925bdaddc4201f984 --platform ethereum
  cg token-price --address 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48,0xdAC17F958D2ee523a2206206994597C13D831ec7 --platform ethereum
  cg token-price --address 0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913 --platform base --vs eur
  cg token-price --address 0x912CE59144191C1204E64559FE8253a0e49E6548 --platform arbitrum-one -o json`,
	RunE: runTokenPrice,
}

func init() {
	tokenPriceCmd.Flags().String("address", "", "Comma-separated contract addresses")
	tokenPriceCmd.Flags().String("platform", "", "Platform ID (e.g. ethereum, base, arbitrum-one, polygon-pos)")
	tokenPriceCmd.Flags().String("vs", "usd", "Target currency")
	rootCmd.AddCommand(tokenPriceCmd)
}

func runTokenPrice(cmd *cobra.Command, args []string) error {
	addressStr, _ := cmd.Flags().GetString("address")
	platform, _ := cmd.Flags().GetString("platform")
	vs, _ := cmd.Flags().GetString("vs")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	if addressStr == "" {
		return fmt.Errorf("provide --address")
	}

	if platform == "" {
		return fmt.Errorf("provide --platform (e.g. ethereum, base, arbitrum-one)")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("/simple/token_price/%s", platform)

	// Short-circuit before any API calls in dry-run mode.
	if isDryRun(cmd) {
		params := map[string]string{
			"contract_addresses":  addressStr,
			"vs_currencies":      vs,
			"include_24hr_change": "true",
		}
		return printDryRun(cfg, "token-price", endpoint, params, nil)
	}

	client := newAPIClient(cfg)
	ctx := cmd.Context()

	addresses := splitTrim(addressStr)
	prices, err := client.SimpleTokenPrice(ctx, platform, addresses, vs)
	if err != nil {
		return err
	}

	if len(prices) == 0 {
		return fmt.Errorf("no valid tokens found")
	}

	if jsonOut {
		return printJSONRaw(prices)
	}

	// Warn about requested addresses that returned no data.
	responseKeys := make(map[string]bool, len(prices))
	for k := range prices {
		responseKeys[strings.ToLower(k)] = true
	}
	for _, addr := range addresses {
		if !responseKeys[strings.ToLower(addr)] {
			warnf("Warning: no data returned for %q\n", addr)
		}
	}

	// Sort response keys for deterministic table output.
	keys := make([]string, 0, len(prices))
	for k := range prices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	headers := []string{"Contract", "Price", "24h Change"}
	var rows [][]string
	for _, addr := range keys {
		data := prices[addr]
		rows = append(rows, []string{
			display.SanitizeCell(addr),
			display.FormatPrice(data[vs], vs),
			display.ColorPercent(data[vs + "_24h_change"]),
		})
	}

	display.PrintTable(headers, rows)
	return nil
}
