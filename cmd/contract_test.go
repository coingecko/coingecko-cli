package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poolSearchJSON builds a pool search response with the given network and address.
// Token relationship IDs follow the "{network}_{address}" convention.
func poolSearchJSON(networkID, addr string) map[string]interface{} {
	return map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":   networkID + "_0xpool",
				"type": "pool",
				"relationships": map[string]interface{}{
					"base_token": map[string]interface{}{
						"data": map[string]string{"id": networkID + "_" + addr, "type": "token"},
					},
					"quote_token": map[string]interface{}{
						"data": map[string]string{"id": networkID + "_0xquote", "type": "token"},
					},
				},
			},
		},
	}
}

// networksJSON builds an onchain networks response mapping networkID to platformID.
func networksJSON(networkID, platformID string) map[string]interface{} {
	return map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":   networkID,
				"type": "network",
				"attributes": map[string]interface{}{
					"name":                        networkID,
					"coingecko_asset_platform_id": platformID,
				},
			},
		},
	}
}

// onchainPriceJSON builds a standard onchain simple token price API response.
func onchainPriceJSON(addr, price, mcap, vol, change, reserve string) map[string]interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"id":   "1",
			"type": "simple_token_price",
			"attributes": map[string]interface{}{
				"token_prices":                map[string]string{addr: price},
				"market_cap_usd":              map[string]string{addr: mcap},
				"h24_volume_usd":              map[string]string{addr: vol},
				"h24_price_change_percentage": map[string]string{addr: change},
				"total_reserve_in_usd":        map[string]string{addr: reserve},
			},
		},
	}
}

func TestContract_MissingAddress(t *testing.T) {
	_, _, err := executeCommand(t, "contract", "--platform", "ethereum", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--address is required")
}

func TestContract_MissingPlatform_TriggersSmartRoute(t *testing.T) {
	// With smart routing, omitting --platform no longer errors immediately;
	// it triggers pool search. This test verifies it reaches the search endpoint.
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			_, _ = w.Write([]byte(`{"data": []}`))
		case "/onchain/networks":
			_, _ = w.Write([]byte(`{"data": []}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", addr, "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pools found")
}

func TestContract_MutualExclusion(t *testing.T) {
	_, _, err := executeCommand(t, "contract", "--address", "0xabc", "--platform", "ethereum", "--onchain", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--platform and --onchain are mutually exclusive")
}

func TestContract_OnchainMissingNetwork_TriggersSmartRoute(t *testing.T) {
	// With smart routing, --onchain without --network triggers pool search
	// to auto-detect the network.
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			_, _ = w.Write([]byte(`{"data": []}`))
		case "/onchain/networks":
			_, _ = w.Write([]byte(`{"data": []}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", addr, "--onchain", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pools found")
}

func TestContract_NetworkWithoutOnchain(t *testing.T) {
	_, _, err := executeCommand(t, "contract", "--address", "0xabc", "--network", "eth", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--network requires --onchain flag")
}

func TestContract_DryRun_Aggregated(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", "0xabc", "--platform", "ethereum", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Contains(t, out.URL, "/simple/token_price/ethereum")
	assert.Equal(t, "0xabc", out.Params["contract_addresses"])
	assert.Equal(t, "usd", out.Params["vs_currencies"])
	assert.Equal(t, "true", out.Params["include_market_cap"])
	assert.Equal(t, "true", out.Params["include_24hr_vol"])
	assert.Equal(t, "true", out.Params["include_24hr_change"])
	assert.Equal(t, "simple-token-price", out.OASOperationID)
	assert.Equal(t, "coingecko-demo.json", out.OASSpec)
}

func TestContract_DryRun_Onchain(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", "0xabc", "--network", "eth", "--onchain", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Contains(t, out.URL, "/onchain/simple/networks/eth/token_price/0xabc")
	assert.Equal(t, "true", out.Params["include_market_cap"])
	assert.Equal(t, "true", out.Params["include_24hr_vol"])
	assert.Equal(t, "true", out.Params["include_24hr_price_change"])
	assert.Equal(t, "true", out.Params["mcap_fdv_fallback"])
	assert.Equal(t, "true", out.Params["include_total_reserve_in_usd"])
	assert.Equal(t, "onchain-simple-price", out.OASOperationID)
	assert.Equal(t, "coingecko-demo.json", out.OASSpec)
	assert.Empty(t, out.Note)
}

func TestContract_DryRun_Onchain_NonUSD(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", "0xabc", "--network", "eth", "--onchain", "--vs", "eur", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.Note, "exchange_rates")
	assert.Contains(t, out.Note, "eur")
}

func TestContract_Aggregated_JSONOutput(t *testing.T) {
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simple/token_price/ethereum", r.URL.Path)
		assert.Equal(t, addr, r.URL.Query().Get("contract_addresses"))
		assert.Equal(t, "usd", r.URL.Query().Get("vs_currencies"))

		resp := map[string]map[string]float64{
			addr: {
				"usd":            1.001,
				"usd_market_cap": 50000000000,
				"usd_24h_vol":    30000000000,
				"usd_24h_change": 0.05,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--platform", "ethereum", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, 1.001, result[addr]["price"])
	assert.Equal(t, float64(50000000000), result[addr]["market_cap"])
	assert.Equal(t, float64(30000000000), result[addr]["volume_24h"])
	assert.Equal(t, 0.05, result[addr]["change_24h"])
}

func TestContract_Aggregated_TableOutput(t *testing.T) {
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			addr: {
				"usd":            1.001,
				"usd_market_cap": 50000000000,
				"usd_24h_vol":    30000000000,
				"usd_24h_change": 2.5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--platform", "ethereum")
	require.NoError(t, err)
	assert.Contains(t, stdout, addr)
	assert.Contains(t, stdout, "Price")
	assert.Contains(t, stdout, "Market Cap")
	assert.Contains(t, stdout, "24h Volume")
	assert.Contains(t, stdout, "24h Change")
}

func TestContract_Aggregated_NoData(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xnonexistent", "--platform", "ethereum", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data returned for address")
}

func TestContract_Onchain_JSONOutput(t *testing.T) {
	addr := "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/eth/token_price/"))
		_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "2289.33", "6692452895.78", "965988358.73", "3.387", "0"))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 2289.33, result[addr]["price"], 0.01)
	assert.InDelta(t, 6692452895.78, result[addr]["market_cap"], 0.01)
	assert.InDelta(t, 965988358.73, result[addr]["volume_24h"], 0.01)
	assert.InDelta(t, 3.387, result[addr]["change_24h"], 0.001)
}

func TestContract_Onchain_NonUSD_Conversion(t *testing.T) {
	addr := "0xaddr"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "2000", "1000000", "500000", "5.0", "0"))
		case r.URL.Path == "/exchange_rates":
			resp := map[string]interface{}{
				"rates": map[string]interface{}{
					"usd": map[string]interface{}{"name": "US Dollar", "unit": "$", "value": 67187.0, "type": "fiat"},
					"eur": map[string]interface{}{"name": "Euro", "unit": "\u20ac", "value": 62345.0, "type": "fiat"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "--vs", "eur", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// factor = 62345 / 67187 ≈ 0.9280
	factor := 62345.0 / 67187.0
	assert.InDelta(t, 2000*factor, result[addr]["price"], 0.01)
	assert.InDelta(t, 1000000*factor, result[addr]["market_cap"], 0.01)
	assert.InDelta(t, 500000*factor, result[addr]["volume_24h"], 0.01)
	// 24h change should be unchanged
	assert.InDelta(t, 5.0, result[addr]["change_24h"], 0.001)
}

func TestContract_Onchain_UnsupportedCurrency(t *testing.T) {
	addr := "0xaddr"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "2000", "1000000", "500000", "5.0", "0"))
		case r.URL.Path == "/exchange_rates":
			resp := map[string]interface{}{
				"rates": map[string]interface{}{
					"usd": map[string]interface{}{"name": "US Dollar", "unit": "$", "value": 67187.0, "type": "fiat"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "--vs", "xyz", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported currency")
}

func TestContract_Onchain_DemoTier(t *testing.T) {
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/onchain/simple/networks/eth/token_price/")
		_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "100.5", "500000", "250000", "2.5", "0"))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 100.5, result[addr]["price"], 0.01)
}

func TestContract_CatalogMetadata(t *testing.T) {
	stdout, _, err := executeCommand(t, "commands")
	require.NoError(t, err)

	var catalog commandCatalog
	require.NoError(t, json.Unmarshal([]byte(stdout), &catalog))

	var contractInfo *commandInfo
	for i := range catalog.Commands {
		if catalog.Commands[i].Name == "contract" {
			contractInfo = &catalog.Commands[i]
			break
		}
	}
	require.NotNil(t, contractInfo, "contract command not in catalog")

	// OAS spec should be present (both modes use demo-compatible endpoints).
	assert.Equal(t, "coingecko-demo.json", contractInfo.OASSpec)

	// Not paid-only — onchain endpoint is available on both demo and paid tiers.
	assert.False(t, contractInfo.PaidOnly)

	// All operation IDs should be present.
	assert.Equal(t, "simple-token-price", contractInfo.OASOperationIDs["default"])
	assert.Equal(t, "onchain-simple-price", contractInfo.OASOperationIDs["--onchain"])
	assert.Equal(t, "search-pools", contractInfo.OASOperationIDs["resolve"])
}

func TestContract_Export_CSV(t *testing.T) {
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			addr: {
				"usd":            1.001,
				"usd_market_cap": 50000000000,
				"usd_24h_vol":    30000000000,
				"usd_24h_change": 0.05,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "contract.csv")

	_, _, err := executeCommand(t, "contract", "--address", addr, "--platform", "ethereum", "--export", csvPath)
	require.NoError(t, err)

	data, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Address")
	assert.Contains(t, content, "Price")
	assert.Contains(t, content, "Market Cap")
	assert.Contains(t, content, addr)
}

// ---------------------------------------------------------------------------
// Smart routing tests
// ---------------------------------------------------------------------------

// poolSearchHandler returns a handler that serves pool search, network list,
// and optionally CG/onchain price endpoints for smart routing tests.
func smartRouteHandler(t *testing.T, addr, networkID, platformID string, cgPrice map[string]float64, onchainAttrs map[string]map[string]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/onchain/search/pools":
			assert.Equal(t, addr, r.URL.Query().Get("query"))
			_ = json.NewEncoder(w).Encode(poolSearchJSON(networkID, addr))

		case r.URL.Path == "/onchain/networks":
			_ = json.NewEncoder(w).Encode(networksJSON(networkID, platformID))

		case strings.HasPrefix(r.URL.Path, "/simple/token_price/"):
			if cgPrice != nil {
				resp := map[string]map[string]float64{addr: cgPrice}
				_ = json.NewEncoder(w).Encode(resp)
			} else {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{})
			}

		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			if onchainAttrs == nil {
				onchainAttrs = map[string]map[string]string{
					"token_prices": {addr: "100.5"},
					"market_cap_usd": {addr: "500000"},
					"h24_volume_usd": {addr: "250000"},
					"h24_price_change_percentage": {addr: "2.5"},
					"total_reserve_in_usd": {addr: "1000000"},
				}
			}
			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"id":         "1",
					"type":       "simple_token_price",
					"attributes": onchainAttrs,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}
}

func TestContract_SmartRoute_AddressOnly(t *testing.T) {
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	cgPrice := map[string]float64{
		"usd":            1.001,
		"usd_market_cap": 50000000000,
		"usd_24h_vol":    30000000000,
		"usd_24h_change": 0.05,
	}

	srv := newTestServer(smartRouteHandler(t, addr, "eth", "ethereum", cgPrice, nil))
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "contract", "--address", addr, "-o", "json")
	require.NoError(t, err)

	// Should have resolved via smart routing.
	assert.Contains(t, stderr, "Resolved address to network=eth, platform=ethereum")

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, 1.001, result[addr]["price"])
	assert.Equal(t, float64(50000000000), result[addr]["market_cap"])
}

func TestContract_SmartRoute_FallbackToOnchain(t *testing.T) {
	addr := "0xabc"

	// CG returns empty (no data), so it should fall back to onchain.
	srv := newTestServer(smartRouteHandler(t, addr, "eth", "ethereum", nil, nil))
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "contract", "--address", addr, "-o", "json")
	require.NoError(t, err)

	assert.Contains(t, stderr, "falling back to onchain")

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 100.5, result[addr]["price"], 0.01)
}

func TestContract_SmartRoute_NoPools(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			_, _ = w.Write([]byte(`{"data": []}`))
		case "/onchain/networks":
			_, _ = w.Write([]byte(`{"data": []}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xnonexistent", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pools found")
	assert.Contains(t, err.Error(), "--platform")
}

func TestContract_SmartRoute_OnchainOnly(t *testing.T) {
	addr := "0xabc"

	// --onchain without --network: should resolve network, then go straight to onchain.
	var cgCalled bool
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/onchain/search/pools":
			_ = json.NewEncoder(w).Encode(poolSearchJSON("eth", addr))

		case r.URL.Path == "/onchain/networks":
			_ = json.NewEncoder(w).Encode(networksJSON("eth", "ethereum"))

		case strings.HasPrefix(r.URL.Path, "/simple/token_price/"):
			cgCalled = true
			t.Fatal("should not call CG endpoint when --onchain is specified")

		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "100.5", "500000", "250000", "2.5", "1000000"))

		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--onchain", "-o", "json")
	require.NoError(t, err)
	assert.False(t, cgCalled)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 100.5, result[addr]["price"], 0.01)
}

func TestContract_SmartRoute_NoPlatformMapping(t *testing.T) {
	addr := "0xabc"

	// Network found but no CG platform mapping — should auto-use onchain.
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/onchain/search/pools":
			_ = json.NewEncoder(w).Encode(poolSearchJSON("obscure-net", addr))

		case r.URL.Path == "/onchain/networks":
			_ = json.NewEncoder(w).Encode(networksJSON("obscure-net", ""))

		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "0.05", "100000", "50000", "1.0", "200000"))

		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, stderr, err := executeCommand(t, "contract", "--address", addr, "-o", "json")
	require.NoError(t, err)

	assert.Contains(t, stderr, "no CG platform mapping")

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 0.05, result[addr]["price"], 0.001)
}

func TestContract_SmartRoute_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	// Address-only dry-run should show the resolution endpoint.
	stdout, _, err := executeCommand(t, "contract", "--address", "0xabc", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Contains(t, out.URL, "/onchain/search/pools")
	assert.Equal(t, "0xabc", out.Params["query"])
	assert.Contains(t, out.Note, "Smart routing")
	assert.Equal(t, "search-pools", out.OASOperationID)
}

// ---------------------------------------------------------------------------
// Reserve/liquidity tests
// ---------------------------------------------------------------------------

func TestContract_Onchain_Reserve(t *testing.T) {
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "100.5", "500000", "250000", "2.5", "1234567.89"))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	// JSON output should include total_reserve.
	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.InDelta(t, 1234567.89, result[addr]["total_reserve"], 0.01)

	// Table output should include Reserve header.
	tableOut, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain")
	require.NoError(t, err)
	assert.Contains(t, tableOut, "Reserve")
}

func TestContract_Onchain_Reserve_CurrencyConversion(t *testing.T) {
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/onchain/simple/networks/"):
			_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "2000", "1000000", "500000", "5.0", "800000"))
		case r.URL.Path == "/exchange_rates":
			resp := map[string]interface{}{
				"rates": map[string]interface{}{
					"usd": map[string]interface{}{"name": "US Dollar", "unit": "$", "value": 67187.0, "type": "fiat"},
					"eur": map[string]interface{}{"name": "Euro", "unit": "€", "value": 62345.0, "type": "fiat"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	})
	defer srv.Close()
	withTestClientPaid(t, srv)

	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "--vs", "eur", "-o", "json")
	require.NoError(t, err)

	var result map[string]map[string]float64
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	factor := 62345.0 / 67187.0
	assert.InDelta(t, 800000*factor, result[addr]["total_reserve"], 0.01)
}

func TestContract_Aggregated_NoReserveColumn(t *testing.T) {
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]map[string]float64{
			addr: {
				"usd":            1.001,
				"usd_market_cap": 50000000000,
				"usd_24h_vol":    30000000000,
				"usd_24h_change": 0.05,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	// Table output should NOT contain Reserve header in aggregated mode.
	stdout, _, err := executeCommand(t, "contract", "--address", addr, "--platform", "ethereum")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "Reserve")

	// JSON output should NOT contain total_reserve in aggregated mode.
	jsonOut, _, err := executeCommand(t, "contract", "--address", addr, "--platform", "ethereum", "-o", "json")
	require.NoError(t, err)
	assert.NotContains(t, jsonOut, "total_reserve")
}

// ---------------------------------------------------------------------------
// resolveAddress error paths
// ---------------------------------------------------------------------------

func TestContract_SmartRoute_PoolSearchError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"status":{"error_code":500,"error_message":"Internal error"}}`))
		case "/onchain/networks":
			_, _ = w.Write([]byte(`{"data": []}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xabc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "searching for address")
}

func TestContract_SmartRoute_EmptyNetworkID(t *testing.T) {
	// Pool found but network relationship has empty ID.
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			// Pool found but neither token matches the queried address.
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"id":   "unknown_0xpool",
						"type": "pool",
						"relationships": map[string]interface{}{
							"base_token": map[string]interface{}{
								"data": map[string]string{"id": "unknown_0xother1", "type": "token"},
							},
							"quote_token": map[string]interface{}{
								"data": map[string]string{"id": "unknown_0xother2", "type": "token"},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/onchain/networks":
			_, _ = w.Write([]byte(`{"data": []}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xabc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not determine network")
}

func TestContract_SmartRoute_NetworksAPIError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/onchain/search/pools":
			_ = json.NewEncoder(w).Encode(poolSearchJSON("eth", "0xabc"))
		case "/onchain/networks":
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"status":{"error_code":500,"error_message":"Internal error"}}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xabc", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching network mappings")
}

// ---------------------------------------------------------------------------
// Onchain no-data path
// ---------------------------------------------------------------------------

func TestContract_Onchain_NoData(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Return response with empty token_prices map.
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "1",
				"type": "simple_token_price",
				"attributes": map[string]interface{}{
					"token_prices":                map[string]string{},
					"market_cap_usd":              map[string]string{},
					"h24_volume_usd":              map[string]string{},
					"h24_price_change_percentage": map[string]string{},
					"total_reserve_in_usd":        map[string]string{},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "contract", "--address", "0xnonexistent", "--network", "eth", "--onchain", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data returned for address")
}

// ---------------------------------------------------------------------------
// CSV export with onchain reserve column
// ---------------------------------------------------------------------------

func TestContract_Export_CSV_Onchain_WithReserve(t *testing.T) {
	addr := "0xabc"
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(onchainPriceJSON(addr, "100.5", "500000", "250000", "2.5", "1234567.89"))
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "onchain.csv")

	_, _, err := executeCommand(t, "contract", "--address", addr, "--network", "eth", "--onchain", "--export", csvPath)
	require.NoError(t, err)

	data, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "Reserve")
	assert.Contains(t, content, addr)
	assert.Contains(t, content, "1234567.89")
}
