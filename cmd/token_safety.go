package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coingecko/coingecko-cli/internal/api"
	"github.com/coingecko/coingecko-cli/internal/display"

	"github.com/spf13/cobra"
)

var tokenSafetyCmd = &cobra.Command{
	Use:   "token-safety <contract_address>",
	Short: "Get safety score and risk info for a token",
	Long: `Fetch safety data for a token using GeckoTerminal's gt_score.

The gt_score (0-100) is a composite safety indicator based on on-chain
signals including liquidity depth, pool age, buy/sell tax detection,
honeypot risk, and token distribution.

Score interpretation:
  80-100  Low risk — established token with deep liquidity
  50-79   Medium risk — review before interacting
  20-49   High risk — proceed with extreme caution
  0-19    Critical risk — likely malicious or abandoned

Uses the public GeckoTerminal API (no API key required).`,
	Example: `  cg token-safety 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48 --network eth
  cg token-safety 0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913 --network base
  cg token-safety <address> --network sol -o json`,
	Args: cobra.ExactArgs(1),
	RunE: runTokenSafety,
}

func init() {
	tokenSafetyCmd.Flags().String("network", "eth", "Blockchain network (eth, base, sol, polygon_pos, arbitrum, bsc, optimism, avalanche)")
	rootCmd.AddCommand(tokenSafetyCmd)
}

// newGTClient is the factory used by command handlers to create GeckoTerminal clients.
// Tests override this to inject httptest-backed clients.
var newGTClient = func() *api.GeckoTerminalClient {
	c := api.NewGeckoTerminalClient()
	c.UserAgent = userAgent
	return c
}

func runTokenSafety(cmd *cobra.Command, args []string) error {
	address := args[0]
	network, _ := cmd.Flags().GetString("network")
	jsonOut := outputJSON(cmd)

	if !jsonOut {
		display.PrintBanner()
	}

	if isDryRun(cmd) {
		endpoint := fmt.Sprintf("/networks/%s/tokens/%s", network, address)
		out := dryRunOutput{
			Method: "GET",
			URL:    "https://api.geckoterminal.com/api/v2" + endpoint,
			Params: map[string]string{
				"network": network,
				"address": address,
			},
			Headers: map[string]string{
				"Accept":     "application/json",
				"User-Agent": userAgent,
			},
			Note: "Uses GeckoTerminal public API (no auth required)",
		}
		if meta, ok := commandMeta["token-safety"]; ok {
			out.OASOperationID = meta.OASOperationID
			out.OASSpec = meta.OASSpec
		}
		return printJSONRaw(out)
	}

	client := newGTClient()
	ctx := cmd.Context()

	// Fetch token info and top pools concurrently.
	type tokenResult struct {
		token *api.GTTokenResponse
		err   error
	}
	type poolsResult struct {
		pools *api.GTPoolsResponse
		err   error
	}

	tokenCh := make(chan tokenResult, 1)
	poolsCh := make(chan poolsResult, 1)

	go func() {
		t, err := client.TokenInfo(ctx, network, address)
		tokenCh <- tokenResult{t, err}
	}()
	go func() {
		p, err := client.TokenPools(ctx, network, address)
		poolsCh <- poolsResult{p, err}
	}()

	tr := <-tokenCh
	pr := <-poolsCh

	if tr.err != nil {
		return fmt.Errorf("fetching token info: %w", tr.err)
	}

	token := tr.token.Data.Attributes

	if jsonOut {
		output := map[string]any{
			"token": map[string]any{
				"name":              token.Name,
				"symbol":            token.Symbol,
				"address":           token.Address,
				"network":           network,
				"price_usd":         token.PriceUSD,
				"fdv_usd":           token.FDVInUSD,
				"volume_24h_usd":    token.Volume24h,
				"gt_score":          token.GTScore,
				"coingecko_coin_id": token.CoingeckoCoinID,
			},
		}
		if pr.err == nil && len(pr.pools.Data) > 0 {
			pools := make([]map[string]any, 0, len(pr.pools.Data))
			for _, p := range pr.pools.Data {
				pools = append(pools, map[string]any{
					"name":           p.Attributes.Name,
					"address":        p.Attributes.Address,
					"gt_score":       p.Attributes.GTScore,
					"reserve_usd":    p.Attributes.ReserveInUSD,
					"volume_24h_usd": p.Attributes.Volume24h.H24,
					"created_at":     p.Attributes.PoolCreatedAt,
				})
			}
			output["pools"] = pools
		}
		return printJSONRaw(output)
	}

	// Table output: Token overview.
	fmt.Println()
	riskLevel, riskColor := classifyGTScore(token.GTScore)
	fmt.Printf("  Token: %s (%s)\n", display.SanitizeCell(token.Name), display.SanitizeCell(token.Symbol))
	fmt.Printf("  Network: %s\n", network)
	fmt.Printf("  Address: %s\n", display.SanitizeCell(token.Address))
	if token.PriceUSD != "" {
		if price, err := strconv.ParseFloat(token.PriceUSD, 64); err == nil {
			fmt.Printf("  Price: %s\n", display.FormatPrice(price))
		}
	}
	if token.FDVInUSD != "" {
		if fdv, err := strconv.ParseFloat(token.FDVInUSD, 64); err == nil {
			fmt.Printf("  FDV: %s\n", display.FormatLargeNumber(fdv))
		}
	}
	fmt.Println()

	// Safety score with color.
	scoreStr := fmt.Sprintf("%.1f / 100", token.GTScore)
	if display.ColorEnabled() {
		scoreStr = riskColor + scoreStr + "\033[0m"
	}
	fmt.Printf("  GT Safety Score: %s  [%s]\n", scoreStr, riskLevel)
	fmt.Println()

	// Pool safety table.
	if pr.err == nil && len(pr.pools.Data) > 0 {
		limit := len(pr.pools.Data)
		if limit > 5 {
			limit = 5
		}

		headers := []string{"Pool", "GT Score", "Risk", "Liquidity", "Volume 24h"}
		rows := make([][]string, 0, limit)
		for _, p := range pr.pools.Data[:limit] {
			pRisk, _ := classifyGTScore(p.Attributes.GTScore)
			liquidity := "-"
			if p.Attributes.ReserveInUSD != "" {
				if v, err := strconv.ParseFloat(p.Attributes.ReserveInUSD, 64); err == nil {
					liquidity = display.FormatLargeNumber(v)
				}
			}
			vol := "-"
			if p.Attributes.Volume24h.H24 != "" {
				if v, err := strconv.ParseFloat(p.Attributes.Volume24h.H24, 64); err == nil {
					vol = display.FormatLargeNumber(v)
				}
			}
			rows = append(rows, []string{
				display.SanitizeCell(truncateName(p.Attributes.Name, 30)),
				fmt.Sprintf("%.1f", p.Attributes.GTScore),
				pRisk,
				liquidity,
				vol,
			})
		}
		fmt.Println("  Top Pools:")
		display.PrintTable(headers, rows)
	}

	if token.CoingeckoCoinID != "" {
		fmt.Printf("\n  CoinGecko: https://www.coingecko.com/en/coins/%s\n", token.CoingeckoCoinID)
	}
	fmt.Printf("  GeckoTerminal: https://www.geckoterminal.com/%s/tokens/%s\n", network, address)
	fmt.Println()

	return nil
}

// classifyGTScore returns a human-readable risk level and ANSI color code.
func classifyGTScore(score float64) (string, string) {
	switch {
	case score >= 80:
		return "Low Risk", "\033[32m"     // green
	case score >= 50:
		return "Medium Risk", "\033[33m"  // yellow
	case score >= 20:
		return "High Risk", "\033[31m"    // red
	default:
		return "Critical Risk", "\033[91m" // bright red
	}
}

func truncateName(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
