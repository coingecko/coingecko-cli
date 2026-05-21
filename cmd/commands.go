package cmd

import (
	"strings"

	"github.com/coingecko/coingecko-cli/internal/ws"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const oasRepo = "https://github.com/coingecko/coingecko-api-oas"

type commandAnnotation struct {
	APIEndpoint     string
	APIEndpoints    map[string]string
	OASOperationID  string
	OASOperationIDs map[string]string
	OASSpec         string
	Transport string // "rest" (default) or "websocket"
	PaidOnly     bool
	RequiresAuth bool
}

var commandMeta = map[string]commandAnnotation{
	"price": {
		APIEndpoint:    "/simple/price",
		OASOperationID: "simple-price",
		OASSpec:        "coingecko-demo.json",
		RequiresAuth:   true,
	},
	"markets": {
		APIEndpoint:    "/coins/markets",
		OASOperationID: "coins-markets",
		OASSpec:        "coingecko-demo.json",
		RequiresAuth:   true,
	},
	"search": {
		APIEndpoint:    "/search",
		OASOperationID: "search-data",
		OASSpec:        "coingecko-demo.json",
		RequiresAuth:   true,
	},
	"trending": {
		APIEndpoint:    "/search/trending",
		OASOperationID: "trending-search",
		OASSpec:        "coingecko-demo.json",
		RequiresAuth:   true,
	},
	"history": {
		APIEndpoints: map[string]string{
			"--date":             "/coins/{id}/history",
			"--days":             "/coins/{id}/market_chart",
			"--days --ohlc":      "/coins/{id}/ohlc",
			"--from/--to":        "/coins/{id}/market_chart/range",
			"--from/--to --ohlc": "/coins/{id}/ohlc/range",
		},
		OASOperationIDs: map[string]string{
			"--date":             "coins-id-history",
			"--days":             "coins-id-market-chart",
			"--days --ohlc":      "coins-id-ohlc",
			"--from/--to":        "coins-id-market-chart-range",
			"--from/--to --ohlc": "coins-id-ohlc-range",
		},
		OASSpec:      "coingecko-demo.json",
		RequiresAuth: true,
	},
	"top-gainers-losers": {
		APIEndpoint:    "/coins/top_gainers_losers",
		OASOperationID: "coins-top-gainers-losers",
		OASSpec:        "coingecko-pro.json",
		PaidOnly:       true,
		RequiresAuth:   true,
	},
	"watch": {
		APIEndpoint:  ws.DefaultWSURL,
		Transport:    "websocket",
		PaidOnly:     true,
		RequiresAuth: true,
	},
	"contract": {
		APIEndpoints: map[string]string{
			"default":   "/simple/token_price/{platform}",
			"--onchain": "/onchain/simple/networks/{network}/token_price/{addresses}",
			"resolve":   "/onchain/search/pools + /onchain/networks",
		},
		OASOperationIDs: map[string]string{
			"default":   "simple-token-price",
			"--onchain": "onchain-simple-price",
			"resolve":   "search-pools",
		},
		OASSpec:      "coingecko-demo.json",
		RequiresAuth: true,
	},
}

type flagInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// validEnum checks if a value is in the validation enums for a given command and flag.
// It checks both catalog enums and internal-only enums.
func validEnum(cmdName, flagName, value string) bool {
	for _, m := range []map[string]map[string][]string{flagEnums, internalEnums} {
		if enums, ok := m[cmdName]; ok {
			if vals, ok := enums[flagName]; ok {
				for _, v := range vals {
					if v == value {
						return true
					}
				}
			}
		}
	}
	return false
}

// internalEnums are used for validation only and not exposed in the command catalog.
// The OHLC "days" enum is stricter than market_chart (which accepts any integer).
var internalEnums = map[string]map[string][]string{
	"history": {
		"days": {"1", "7", "14", "30", "90", "180", "365", "max"},
	},
}

// Flag enums sourced from CoinGecko OAS spec. Exposed in the command catalog.
var flagEnums = map[string]map[string][]string{
	"history": {
		"interval": {"hourly", "daily"},
	},
	"top-gainers-losers": {
		"duration":  {"1h", "24h", "7d", "14d", "30d", "60d", "1y"},
		"top-coins": {"300", "500", "1000", "all"},
	},
	"markets": {
		"order": {"market_cap_desc", "market_cap_asc", "volume_asc", "volume_desc", "id_asc", "id_desc"},
	},
}

type commandInfo struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Flags           []flagInfo        `json:"flags"`
	Examples        []string          `json:"examples,omitempty"`
	OutputFormats   []string          `json:"output_formats"`
	RequiresAuth    bool              `json:"requires_auth"`
	PaidOnly bool `json:"paid_only"`
	Transport       string            `json:"transport,omitempty"`
	APIEndpoint     string            `json:"api_endpoint,omitempty"`
	APIEndpoints    map[string]string `json:"api_endpoints,omitempty"`
	OASOperationID  string            `json:"oas_operation_id,omitempty"`
	OASOperationIDs map[string]string `json:"oas_operation_ids,omitempty"`
	OASSpec string `json:"oas_spec,omitempty"`
}

type commandCatalog struct {
	Version  string        `json:"version"`
	OASRepo  string        `json:"oas_repo"`
	Commands []commandInfo `json:"commands"`
}

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "Output machine-readable command catalog (JSON)",
	RunE:  runCommands,
}

func init() {
	rootCmd.AddCommand(commandsCmd)
}

func runCommands(cmd *cobra.Command, args []string) error {
	catalog := commandCatalog{
		Version: version,
		OASRepo: oasRepo,
	}

	for _, c := range rootCmd.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}

		// Skip non-data commands.
		if c.Name() == "auth" || c.Name() == "status" || c.Name() == "version" {
			info := commandInfo{
				Name:          c.Name(),
				Description:   c.Short,
				Flags:         extractFlags(c),
				Examples:      splitExamples(c.Example),
				OutputFormats: []string{"table"},
			}
			if c.Name() == "version" {
				info.OutputFormats = []string{"table", "json"}
			}
			catalog.Commands = append(catalog.Commands, info)
			continue
		}

		// Handle subcommands (tui markets, tui trending).
		if c.HasSubCommands() {
			for _, sub := range c.Commands() {
				if sub.Hidden {
					continue
				}
				info := commandInfo{
					Name:          c.Name() + " " + sub.Name(),
					Description:   sub.Short,
					Flags:         extractFlags(sub),
					Examples:      splitExamples(sub.Example),
					OutputFormats: []string{"tui"},
					RequiresAuth:  true,
				}
				catalog.Commands = append(catalog.Commands, info)
			}
			continue
		}

		info := commandInfo{
			Name:          c.Name(),
			Description:   c.Short,
			Flags:         extractFlags(c),
			Examples:      splitExamples(c.Example),
			OutputFormats: []string{"table", "json"},
			RequiresAuth:  true,
		}

		if meta, ok := commandMeta[c.Name()]; ok {
			info.PaidOnly = meta.PaidOnly
			info.RequiresAuth = meta.RequiresAuth
			info.Transport = meta.Transport
			info.APIEndpoint = meta.APIEndpoint
			info.APIEndpoints = meta.APIEndpoints
			info.OASOperationID = meta.OASOperationID
			info.OASOperationIDs = meta.OASOperationIDs
			info.OASSpec = meta.OASSpec
		}

		catalog.Commands = append(catalog.Commands, info)
	}

	return printJSONRaw(catalog)
}

func extractFlags(cmd *cobra.Command) []flagInfo {
	var flags []flagInfo
	cmd.NonInheritedFlags().VisitAll(func(f *pflag.Flag) {
		fi := flagInfo{
			Name:        f.Name,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		}
		if enums, ok := flagEnums[cmd.Name()]; ok {
			if e, ok := enums[f.Name]; ok {
				fi.Enum = e
			}
		}
		flags = append(flags, fi)
	})
	return flags
}

func splitExamples(s string) []string {
	if s == "" {
		return nil
	}
	var examples []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimLeft(line, " ")
		if line != "" {
			examples = append(examples, line)
		}
	}
	return examples
}
