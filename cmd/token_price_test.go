package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/coingecko/coingecko-cli/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenPrice_MissingAddress(t *testing.T) {
	_, _, err := executeCommand(t, "token-price", "--platform", "ethereum", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--address")
}

func TestTokenPrice_MissingPlatform(t *testing.T) {
	_, _, err := executeCommand(t, "token-price", "--address", "0x1234", "-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--platform")
}

func TestTokenPrice_DryRun(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP call in dry-run mode")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "token-price",
		"--address", "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984",
		"--platform", "ethereum",
		"--dry-run", "-o", "json")
	require.NoError(t, err)

	var out dryRunOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "GET", out.Method)
	assert.Equal(t, "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984", out.Params["contract_addresses"])
	assert.Contains(t, out.URL, "/simple/token_price/ethereum")
}

func TestTokenPrice_JSONOutput(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simple/token_price/ethereum", r.URL.Path)
		assert.Equal(t, "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984", r.URL.Query().Get("contract_addresses"))
		resp := api.PriceResponse{
			"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984": {"usd": 7.42, "usd_24h_change": -1.3},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "token-price",
		"--address", "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984",
		"--platform", "ethereum",
		"-o", "json")
	require.NoError(t, err)

	var prices api.PriceResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &prices))
	assert.Equal(t, 7.42, prices["0x1f9840a85d5af5bf1d1762f925bdaddc4201f984"]["usd"])
}

func TestTokenPrice_MultipleAddresses(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("contract_addresses"), ",")
		resp := api.PriceResponse{
			"0xaaa": {"usd": 1.0, "usd_24h_change": 0.5},
			"0xbbb": {"usd": 2.0, "usd_24h_change": -0.5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	stdout, _, err := executeCommand(t, "token-price",
		"--address", "0xaaa,0xbbb",
		"--platform", "ethereum",
		"-o", "json")
	require.NoError(t, err)

	var prices api.PriceResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &prices))
	assert.Len(t, prices, 2)
}

func TestTokenPrice_NoResults(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(api.PriceResponse{})
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, _, err := executeCommand(t, "token-price",
		"--address", "0xnotreal",
		"--platform", "ethereum",
		"-o", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid tokens found")
}

func TestTokenPrice_PartialMiss_WarnsOnStderr(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := api.PriceResponse{
			"0xaaa": {"usd": 1.0, "usd_24h_change": 0.5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	_, stderr, err := executeCommand(t, "token-price",
		"--address", "0xaaa,0xmissing",
		"--platform", "ethereum")
	require.NoError(t, err)
	assert.Contains(t, stderr, `no data returned for "0xmissing"`)
}
