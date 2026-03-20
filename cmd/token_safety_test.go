package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coingecko/coingecko-cli/internal/api"
	"github.com/coingecko/coingecko-cli/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestGTClient(t *testing.T, srv *httptest.Server) {
	t.Helper()

	// Override loadConfig to return a test config (token-safety doesn't need auth,
	// but other setup code may call it).
	origLoad := loadConfig
	loadConfig = func() (*config.Config, error) {
		return &config.Config{APIKey: "test-key", Tier: "demo"}, nil
	}
	t.Cleanup(func() { loadConfig = origLoad })

	origGT := newGTClient
	newGTClient = func() *api.GeckoTerminalClient {
		c := api.NewGeckoTerminalClient()
		c.SetBaseURL(srv.URL)
		return c
	}
	t.Cleanup(func() { newGTClient = origGT })
}

func TestTokenSafety_MissingAddress(t *testing.T) {
	_, _, err := executeCommand(t, "token-safety", "-o", "json")
	require.Error(t, err)
}

func TestTokenSafety_DryRun(t *testing.T) {
	stdout, _, err := executeCommand(t, "token-safety", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Contains(t, out.URL, "geckoterminal.com")
	assert.Contains(t, out.URL, "/networks/eth/tokens/0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
}

func TestTokenSafety_JSONOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/networks/eth/tokens/0xtest", func(w http.ResponseWriter, r *http.Request) {
		resp := api.GTTokenResponse{
			Data: api.GTTokenData{
				ID:   "eth_0xtest",
				Type: "token",
				Attributes: api.GTTokenAttrs{
					Name:            "Test Token",
					Symbol:          "TEST",
					Address:         "0xtest",
					PriceUSD:        "1.23",
					FDVInUSD:        "1000000",
					GTScore:         85.5,
					CoingeckoCoinID: "test-token",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/networks/eth/tokens/0xtest/pools", func(w http.ResponseWriter, r *http.Request) {
		resp := api.GTPoolsResponse{
			Data: []api.GTPoolData{
				{
					ID:   "eth_pool1",
					Type: "pool",
					Attributes: api.GTPoolAttrs{
						Name:         "TEST / WETH",
						Address:      "0xpool1",
						GTScore:      82.0,
						ReserveInUSD: "500000",
						Volume24h:    api.GTVolumeData{H24: "100000"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	withTestGTClient(t, srv)

	stdout, _, err := executeCommand(t, "token-safety", "0xtest", "-o", "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	token, ok := result["token"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Test Token", token["name"])
	assert.Equal(t, "TEST", token["symbol"])
	assert.Equal(t, 85.5, token["gt_score"])

	pools, ok := result["pools"].([]any)
	require.True(t, ok)
	assert.Len(t, pools, 1)
}

func TestTokenSafety_Network(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/networks/base/tokens/0xbase", func(w http.ResponseWriter, r *http.Request) {
		resp := api.GTTokenResponse{
			Data: api.GTTokenData{
				Attributes: api.GTTokenAttrs{
					Name:    "Base Token",
					Symbol:  "BASE",
					Address: "0xbase",
					GTScore: 45.0,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/networks/base/tokens/0xbase/pools", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.GTPoolsResponse{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	withTestGTClient(t, srv)

	stdout, _, err := executeCommand(t, "token-safety", "0xbase", "--network", "base", "-o", "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	token := result["token"].(map[string]any)
	assert.Equal(t, "base", token["network"])
	assert.Equal(t, 45.0, token["gt_score"])
}

func TestClassifyGTScore(t *testing.T) {
	tests := []struct {
		score float64
		level string
	}{
		{95, "Low Risk"},
		{80, "Low Risk"},
		{65, "Medium Risk"},
		{50, "Medium Risk"},
		{35, "High Risk"},
		{20, "High Risk"},
		{10, "Critical Risk"},
		{0, "Critical Risk"},
	}
	for _, tt := range tests {
		level, _ := classifyGTScore(tt.score)
		assert.Equal(t, tt.level, level, "score=%v", tt.score)
	}
}
