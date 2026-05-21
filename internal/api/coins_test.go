package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/coingecko/coingecko-cli/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPaidClient creates a Client with paid tier for testing paid-only endpoints.
func testPaidClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	cfg := &config.Config{APIKey: "test-key", Tier: config.TierPaid}
	c := NewClient(cfg)
	c.SetBaseURL(srv.URL)
	return c, srv
}

// ---------------------------------------------------------------------------
// SimplePrice
// ---------------------------------------------------------------------------

func TestSimplePrice(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simple/price", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "bitcoin,ethereum", q.Get("ids"))
		assert.Equal(t, "usd", q.Get("vs_currencies"))
		assert.Equal(t, "true", q.Get("include_24hr_change"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"bitcoin":  {"usd": 65000, "usd_24h_change": 2.5},
			"ethereum": {"usd": 3400,  "usd_24h_change": -1.2}
		}`))
	})
	defer srv.Close()

	result, err := c.SimplePrice(context.Background(), []string{"bitcoin", "ethereum"}, "usd")
	require.NoError(t, err)
	assert.Equal(t, float64(65000), result["bitcoin"]["usd"])
	assert.Equal(t, 2.5, result["bitcoin"]["usd_24h_change"])
	assert.Equal(t, float64(3400), result["ethereum"]["usd"])
	assert.Equal(t, -1.2, result["ethereum"]["usd_24h_change"])
}

// ---------------------------------------------------------------------------
// CoinMarkets
// ---------------------------------------------------------------------------

func TestCoinMarkets(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/markets", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "100", q.Get("per_page"))
		assert.Equal(t, "1", q.Get("page"))
		assert.Equal(t, "market_cap_desc", q.Get("order"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			{"id":"bitcoin","symbol":"btc","name":"Bitcoin","current_price":65000,"market_cap_rank":1}
		]`))
	})
	defer srv.Close()

	result, err := c.CoinMarkets(context.Background(), "usd", 100, 1, "market_cap_desc", "")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "bitcoin", result[0].ID)
	assert.Equal(t, "btc", result[0].Symbol)
	assert.Equal(t, float64(65000), result[0].CurrentPrice)
	assert.Equal(t, 1, result[0].MarketCapRank)
}

func TestCoinMarkets_WithCategory(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "layer-1", q.Get("category"))

		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	result, err := c.CoinMarkets(context.Background(), "usd", 50, 1, "market_cap_desc", "layer-1")
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestCoinMarkets_NoCategoryParam(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		// When category is empty, the param should not be present
		assert.Empty(t, r.URL.Query().Get("category"))
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	_, err := c.CoinMarkets(context.Background(), "usd", 50, 1, "market_cap_desc", "")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "bitcoin", r.URL.Query().Get("query"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"coins": [
				{"id":"bitcoin","name":"Bitcoin","symbol":"btc","market_cap_rank":1},
				{"id":"bitcoin-cash","name":"Bitcoin Cash","symbol":"bch","market_cap_rank":20}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.Search(context.Background(), "bitcoin")
	require.NoError(t, err)
	require.Len(t, result.Coins, 2)
	assert.Equal(t, "bitcoin", result.Coins[0].ID)
	assert.Equal(t, "Bitcoin", result.Coins[0].Name)
	assert.Equal(t, "btc", result.Coins[0].Symbol)
	assert.Equal(t, 1, result.Coins[0].MarketCapRank)
	assert.Equal(t, "bitcoin-cash", result.Coins[1].ID)
}

// ---------------------------------------------------------------------------
// SearchTrending
// ---------------------------------------------------------------------------

func TestSearchTrending(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/trending", r.URL.Path)
		// No show_max param when empty
		assert.Empty(t, r.URL.Query().Get("show_max"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"coins": [{"item":{"id":"pepe","name":"Pepe","symbol":"pepe","market_cap_rank":50,"score":0}}],
			"nfts": [{"id":"bored-apes","name":"Bored Apes","symbol":"BAYC","floor_price_24h_percentage_change":5.2}],
			"categories": [{"id":1,"name":"Meme","market_cap_1h_change":3.1}]
		}`))
	})
	defer srv.Close()

	result, err := c.SearchTrending(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, result.Coins, 1)
	assert.Equal(t, "pepe", result.Coins[0].Item.ID)
	assert.Equal(t, "Pepe", result.Coins[0].Item.Name)
	require.Len(t, result.NFTs, 1)
	assert.Equal(t, "bored-apes", result.NFTs[0].ID)
	assert.Equal(t, 5.2, result.NFTs[0].FloorPriceInUSD24hPC)
	require.Len(t, result.Categories, 1)
	assert.Equal(t, "Meme", result.Categories[0].Name)
	assert.Equal(t, 3.1, result.Categories[0].MarketCap1hChange)
}

func TestSearchTrending_WithShowMax(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/trending", r.URL.Path)
		assert.Equal(t, "30", r.URL.Query().Get("show_max"))

		_, _ = w.Write([]byte(`{"coins":[],"nfts":[],"categories":[]}`))
	})
	defer srv.Close()

	result, err := c.SearchTrending(context.Background(), "30")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// CoinHistory
// ---------------------------------------------------------------------------

func TestCoinHistory(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/bitcoin/history", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "01-01-2025", q.Get("date"))
		assert.Equal(t, "false", q.Get("localization"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"id":"bitcoin","symbol":"btc","name":"Bitcoin",
			"market_data":{
				"current_price":{"usd":42000},
				"market_cap":{"usd":800000000000},
				"total_volume":{"usd":30000000000}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.CoinHistory(context.Background(), "bitcoin", "01-01-2025")
	require.NoError(t, err)
	assert.Equal(t, "bitcoin", result.ID)
	assert.Equal(t, "btc", result.Symbol)
	require.NotNil(t, result.MarketData)
	assert.Equal(t, float64(42000), result.MarketData.CurrentPrice["usd"])
	assert.Equal(t, float64(800000000000), result.MarketData.MarketCap["usd"])
}

// ---------------------------------------------------------------------------
// CoinMarketChart
// ---------------------------------------------------------------------------

func TestCoinMarketChart(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/bitcoin/market_chart", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "7", q.Get("days"))
		assert.Empty(t, q.Get("interval")) // no interval when empty

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"prices":[[1700000000000,65000],[1700086400000,65500]],
			"market_caps":[[1700000000000,1200000000000]],
			"total_volumes":[[1700000000000,40000000000]]
		}`))
	})
	defer srv.Close()

	result, err := c.CoinMarketChart(context.Background(), "bitcoin", "usd", "7", "")
	require.NoError(t, err)
	require.Len(t, result.Prices, 2)
	assert.Equal(t, float64(65000), result.Prices[0][1])
	assert.Equal(t, float64(65500), result.Prices[1][1])
	require.Len(t, result.MarketCaps, 1)
	require.Len(t, result.TotalVolumes, 1)
}

func TestCoinMarketChart_WithInterval(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "daily", r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`{"prices":[],"market_caps":[],"total_volumes":[]}`))
	})
	defer srv.Close()

	result, err := c.CoinMarketChart(context.Background(), "bitcoin", "usd", "30", "daily")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// CoinMarketChartRange
// ---------------------------------------------------------------------------

func TestCoinMarketChartRange(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/ethereum/market_chart/range", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "1700000000", q.Get("from"))
		assert.Equal(t, "1700900000", q.Get("to"))
		assert.Empty(t, q.Get("interval"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"prices":[[1700000000000,3400]],
			"market_caps":[[1700000000000,400000000000]],
			"total_volumes":[[1700000000000,20000000000]]
		}`))
	})
	defer srv.Close()

	result, err := c.CoinMarketChartRange(context.Background(), "ethereum", "usd", 1700000000, 1700900000, "")
	require.NoError(t, err)
	require.Len(t, result.Prices, 1)
	assert.Equal(t, float64(3400), result.Prices[0][1])
}

func TestCoinMarketChartRange_WithInterval(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "hourly", r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`{"prices":[],"market_caps":[],"total_volumes":[]}`))
	})
	defer srv.Close()

	result, err := c.CoinMarketChartRange(context.Background(), "bitcoin", "usd", 1700000000, 1700900000, "hourly")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// CoinOHLC
// ---------------------------------------------------------------------------

func TestCoinOHLC(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/bitcoin/ohlc", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "7", q.Get("days"))
		assert.Empty(t, q.Get("interval"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			[1700000000000, 64000, 66000, 63500, 65000],
			[1700086400000, 65000, 67000, 64500, 66500]
		]`))
	})
	defer srv.Close()

	result, err := c.CoinOHLC(context.Background(), "bitcoin", "usd", "7", "")
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Len(t, result[0], 5)
	assert.Equal(t, float64(64000), result[0][1]) // open
	assert.Equal(t, float64(66000), result[0][2]) // high
	assert.Equal(t, float64(63500), result[0][3]) // low
	assert.Equal(t, float64(65000), result[0][4]) // close
}

func TestCoinOHLC_WithInterval(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "daily", r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	result, err := c.CoinOHLC(context.Background(), "bitcoin", "usd", "30", "daily")
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

// ---------------------------------------------------------------------------
// CoinOHLCRange (paid only)
// ---------------------------------------------------------------------------

func TestCoinOHLCRange_DemoRestricted(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach the server with a demo client")
	})
	defer srv.Close()

	_, err := c.CoinOHLCRange(context.Background(), "bitcoin", "usd", 1700000000, 1700900000, "daily")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPlanRestricted)
}

func TestCoinOHLCRange_Paid(t *testing.T) {
	c, srv := testPaidClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/bitcoin/ohlc/range", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "1700000000", q.Get("from"))
		assert.Equal(t, "1700900000", q.Get("to"))
		assert.Equal(t, "daily", q.Get("interval"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			[1700000000000, 64000, 66000, 63500, 65000]
		]`))
	})
	defer srv.Close()

	result, err := c.CoinOHLCRange(context.Background(), "bitcoin", "usd", 1700000000, 1700900000, "daily")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, float64(64000), result[0][1])
}

func TestCoinOHLCRange_PaidNoInterval(t *testing.T) {
	c, srv := testPaidClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	result, err := c.CoinOHLCRange(context.Background(), "bitcoin", "usd", 1700000000, 1700900000, "")
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

// ---------------------------------------------------------------------------
// TopGainersLosers (paid only)
// ---------------------------------------------------------------------------

func TestTopGainersLosers_DemoRestricted(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach the server with a demo client")
	})
	defer srv.Close()

	_, err := c.TopGainersLosers(context.Background(), "usd", "24h", "1000", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPlanRestricted)
}

func TestTopGainersLosers_Paid(t *testing.T) {
	c, srv := testPaidClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/top_gainers_losers", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "usd", q.Get("vs_currency"))
		assert.Equal(t, "24h", q.Get("duration"))
		assert.Equal(t, "1000", q.Get("top_coins"))
		assert.Empty(t, q.Get("price_change_percentage"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"top_gainers":[
				{"id":"coin-a","symbol":"a","name":"Coin A","market_cap_rank":10,"usd":1.5,"usd_24h_change":50.0}
			],
			"top_losers":[
				{"id":"coin-b","symbol":"b","name":"Coin B","market_cap_rank":20,"usd":0.5,"usd_24h_change":-40.0}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.TopGainersLosers(context.Background(), "usd", "24h", "1000", "")
	require.NoError(t, err)
	require.Len(t, result.TopGainers, 1)
	assert.Equal(t, "coin-a", result.TopGainers[0].ID)
	assert.Equal(t, "a", result.TopGainers[0].Symbol)
	assert.Equal(t, 10, result.TopGainers[0].MarketCapRank)
	assert.Equal(t, 1.5, result.TopGainers[0].Price("usd"))
	assert.Equal(t, 50.0, result.TopGainers[0].PriceChange("usd"))
	require.Len(t, result.TopLosers, 1)
	assert.Equal(t, "coin-b", result.TopLosers[0].ID)
	assert.Equal(t, -40.0, result.TopLosers[0].PriceChange("usd"))
}

func TestTopGainersLosers_WithPriceChangePct(t *testing.T) {
	c, srv := testPaidClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1h", r.URL.Query().Get("price_change_percentage"))
		_, _ = w.Write([]byte(`{"top_gainers":[],"top_losers":[]}`))
	})
	defer srv.Close()

	result, err := c.TopGainersLosers(context.Background(), "usd", "24h", "1000", "1h")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// CoinDetail
// ---------------------------------------------------------------------------

func TestCoinDetail(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/coins/bitcoin", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "false", q.Get("localization"))
		assert.Equal(t, "false", q.Get("tickers"))
		assert.Equal(t, "false", q.Get("community_data"))
		assert.Equal(t, "false", q.Get("developer_data"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"id":"bitcoin","symbol":"btc","name":"Bitcoin","market_cap_rank":1,
			"description":{"en":"Bitcoin is a cryptocurrency."},
			"market_data":{
				"current_price":{"usd":65000},
				"market_cap":{"usd":1200000000000},
				"total_volume":{"usd":40000000000},
				"high_24h":{"usd":66000},
				"low_24h":{"usd":64000},
				"price_change_percentage_24h":2.5,
				"ath":{"usd":69000},
				"ath_change_percentage":{"usd":-5.8},
				"ath_date":{"usd":"2021-11-10T14:24:11.849Z"},
				"atl":{"usd":67.81},
				"atl_change_percentage":{"usd":95000},
				"atl_date":{"usd":"2013-07-06T00:00:00.000Z"},
				"circulating_supply":19500000,
				"total_supply":21000000
			}
		}`))
	})
	defer srv.Close()

	result, err := c.CoinDetail(context.Background(), "bitcoin")
	require.NoError(t, err)
	assert.Equal(t, "bitcoin", result.ID)
	assert.Equal(t, "btc", result.Symbol)
	assert.Equal(t, "Bitcoin", result.Name)
	assert.Equal(t, 1, result.MarketCapRank)
	assert.Equal(t, "Bitcoin is a cryptocurrency.", result.Description.EN)
	require.NotNil(t, result.MarketData)
	assert.Equal(t, float64(65000), result.MarketData.CurrentPrice["usd"])
	assert.Equal(t, float64(66000), result.MarketData.High24h["usd"])
	assert.Equal(t, float64(64000), result.MarketData.Low24h["usd"])
	assert.Equal(t, 2.5, result.MarketData.PriceChangePercentage24h)
	assert.Equal(t, float64(21000000), result.MarketData.TotalSupply)
	assert.Equal(t, float64(19500000), result.MarketData.CirculatingSupply)
}

func TestCoinDetail_PathEscaping(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		// Verify the path is properly escaped for IDs with special characters
		assert.Equal(t, "/coins/coin%2Fwith-slash", r.URL.RawPath)
		_, _ = w.Write([]byte(`{"id":"coin/with-slash","symbol":"x","name":"X"}`))
	})
	defer srv.Close()

	result, err := c.CoinDetail(context.Background(), "coin/with-slash")
	require.NoError(t, err)
	assert.Equal(t, "coin/with-slash", result.ID)
}

// ---------------------------------------------------------------------------
// SimpleTokenPrice
// ---------------------------------------------------------------------------

func TestSimpleTokenPrice_Success(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simple/token_price/ethereum", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "0xdac17f958d2ee523a2206206994597c13d831ec7", q.Get("contract_addresses"))
		assert.Equal(t, "usd", q.Get("vs_currencies"))
		assert.Equal(t, "true", q.Get("include_market_cap"))
		assert.Equal(t, "true", q.Get("include_24hr_vol"))
		assert.Equal(t, "true", q.Get("include_24hr_change"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"0xdac17f958d2ee523a2206206994597c13d831ec7": {
				"usd": 1.001,
				"usd_market_cap": 50000000000,
				"usd_24h_vol": 30000000000,
				"usd_24h_change": 0.05
			}
		}`))
	})
	defer srv.Close()

	result, err := c.SimpleTokenPrice(context.Background(), "ethereum", []string{"0xdac17f958d2ee523a2206206994597c13d831ec7"}, "usd")
	require.NoError(t, err)
	addr := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	assert.Equal(t, 1.001, result[addr]["usd"])
	assert.Equal(t, float64(50000000000), result[addr]["usd_market_cap"])
	assert.Equal(t, float64(30000000000), result[addr]["usd_24h_vol"])
	assert.Equal(t, 0.05, result[addr]["usd_24h_change"])
}

func TestSimpleTokenPrice_PathEncoding(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/simple/token_price/binance-smart-chain", r.URL.Path)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	_, err := c.SimpleTokenPrice(context.Background(), "binance-smart-chain", []string{"0xaddr"}, "usd")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// OnchainSimpleTokenPrice
// ---------------------------------------------------------------------------

func TestOnchainSimpleTokenPrice_Success(t *testing.T) {
	c, srv := testPaidClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/onchain/simple/networks/eth/token_price/0xaddr", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "true", q.Get("include_market_cap"))
		assert.Equal(t, "true", q.Get("include_24hr_vol"))
		assert.Equal(t, "true", q.Get("include_24hr_price_change"))
		assert.Equal(t, "true", q.Get("mcap_fdv_fallback"))
		assert.Equal(t, "true", q.Get("include_total_reserve_in_usd"))

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "123",
				"type": "simple_token_price",
				"attributes": {
					"token_prices": {"0xaddr": "2289.33"},
					"market_cap_usd": {"0xaddr": "6692452895.779648"},
					"h24_volume_usd": {"0xaddr": "965988358.733808"},
					"h24_price_change_percentage": {"0xaddr": "3.387"},
					"total_reserve_in_usd": {"0xaddr": "12345678.90"}
				}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.OnchainSimpleTokenPrice(context.Background(), "eth", []string{"0xaddr"})
	require.NoError(t, err)
	assert.Equal(t, "2289.33", result.Data.Attributes.TokenPrices["0xaddr"])
	assert.Equal(t, "6692452895.779648", result.Data.Attributes.MarketCapUSD["0xaddr"])
	assert.Equal(t, "965988358.733808", result.Data.Attributes.H24VolumeUSD["0xaddr"])
	assert.Equal(t, "3.387", result.Data.Attributes.H24PriceChangePct["0xaddr"])
	assert.Equal(t, "12345678.90", result.Data.Attributes.TotalReserveInUSD["0xaddr"])
}

func TestOnchainSimpleTokenPrice_DemoTier(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/onchain/simple/networks/eth/token_price/0xaddr")
		resp := `{"data":{"id":"1","type":"simple_token_price","attributes":{"token_prices":{"0xaddr":"100.5"},"market_cap_usd":{},"h24_volume_usd":{},"h24_price_change_percentage":{}}}}`
		_, _ = w.Write([]byte(resp))
	})
	defer srv.Close()

	result, err := c.OnchainSimpleTokenPrice(context.Background(), "eth", []string{"0xaddr"})
	require.NoError(t, err)
	assert.Equal(t, "100.5", result.Data.Attributes.TokenPrices["0xaddr"])
}

// ---------------------------------------------------------------------------
// ExchangeRates
// ---------------------------------------------------------------------------

func TestExchangeRates_Success(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/exchange_rates", r.URL.Path)

		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"rates": {
				"btc": {"name": "Bitcoin", "unit": "BTC", "value": 1.0, "type": "crypto"},
				"usd": {"name": "US Dollar", "unit": "$", "value": 67187.0, "type": "fiat"},
				"eur": {"name": "Euro", "unit": "\u20ac", "value": 62345.0, "type": "fiat"}
			}
		}`))
	})
	defer srv.Close()

	result, err := c.ExchangeRates(context.Background())
	require.NoError(t, err)
	assert.Equal(t, float64(1), result.Rates["btc"].Value)
	assert.Equal(t, float64(67187), result.Rates["usd"].Value)
	assert.Equal(t, "US Dollar", result.Rates["usd"].Name)
	assert.Equal(t, "$", result.Rates["usd"].Unit)
	assert.Equal(t, "fiat", result.Rates["usd"].Type)
	assert.Equal(t, float64(62345), result.Rates["eur"].Value)
}

// ---------------------------------------------------------------------------
// OnchainSearchPools
// ---------------------------------------------------------------------------

func TestOnchainSearchPools_Success(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/onchain/search/pools", r.URL.Path)
		assert.Equal(t, "0xaddr", r.URL.Query().Get("query"))

		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "eth_0xpool1",
				"type": "pool",
				"relationships": {
					"base_token": {
						"data": {"id": "eth_0xaddr", "type": "token"}
					},
					"quote_token": {
						"data": {"id": "eth_0xquote", "type": "token"}
					}
				}
			}]
		}`))
	})
	defer srv.Close()

	result, err := c.OnchainSearchPools(context.Background(), "0xaddr")
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, "eth_0xaddr", result.Data[0].Relationships.BaseToken.Data.ID)
}

func TestOnchainSearchPools_NoResults(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": []}`))
	})
	defer srv.Close()

	result, err := c.OnchainSearchPools(context.Background(), "0xnonexistent")
	require.NoError(t, err)
	assert.Empty(t, result.Data)
}

// ---------------------------------------------------------------------------
// OnchainNetworks
// ---------------------------------------------------------------------------

func TestOnchainNetworks_Success(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/onchain/networks", r.URL.Path)

		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "eth",
					"type": "network",
					"attributes": {
						"name": "Ethereum",
						"coingecko_asset_platform_id": "ethereum"
					}
				},
				{
					"id": "bsc",
					"type": "network",
					"attributes": {
						"name": "BNB Chain",
						"coingecko_asset_platform_id": "binance-smart-chain"
					}
				}
			]
		}`))
	})
	defer srv.Close()

	result, err := c.OnchainNetworks(context.Background())
	require.NoError(t, err)
	require.Len(t, result.Data, 2)
	assert.Equal(t, "eth", result.Data[0].ID)
	assert.Equal(t, "ethereum", result.Data[0].Attributes.CoingeckoAssetPlatformID)
	assert.Equal(t, "bsc", result.Data[1].ID)
	assert.Equal(t, "binance-smart-chain", result.Data[1].Attributes.CoingeckoAssetPlatformID)
}

// ---------------------------------------------------------------------------
// FetchAllMarkets (existing tests kept below)
// ---------------------------------------------------------------------------

func TestFetchAllMarkets_SinglePage(t *testing.T) {
	var calls int32
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "250", r.URL.Query().Get("per_page"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		coins := make([]MarketCoin, 10)
		for i := range coins {
			coins[i] = MarketCoin{ID: fmt.Sprintf("coin-%d", i), MarketCapRank: i + 1}
		}
		_ = json.NewEncoder(w).Encode(coins)
	})
	defer srv.Close()

	result, err := c.FetchAllMarkets(context.Background(), "usd", 10, "market_cap_desc", "")
	require.NoError(t, err)
	assert.Len(t, result, 10)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestFetchAllMarkets_MultiPage(t *testing.T) {
	var calls int32
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		atomic.AddInt32(&calls, 1)
		// Return 250 coins per page (full page)
		coins := make([]MarketCoin, 250)
		for i := range coins {
			pageNum := 1
			if page == "2" {
				pageNum = 2
			}
			coins[i] = MarketCoin{
				ID:            fmt.Sprintf("coin-p%s-%d", page, i),
				MarketCapRank: (pageNum-1)*250 + i + 1,
			}
		}
		_ = json.NewEncoder(w).Encode(coins)
	})
	defer srv.Close()

	result, err := c.FetchAllMarkets(context.Background(), "usd", 300, "market_cap_desc", "")
	require.NoError(t, err)
	// Requested 300, should trim from 500 (2 pages × 250)
	assert.Len(t, result, 300)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	// First coin from page 1, last from page 2
	assert.Equal(t, "coin-p1-0", result[0].ID)
	assert.Equal(t, "coin-p2-49", result[299].ID)
}

func TestFetchAllMarkets_PartialLastPage(t *testing.T) {
	var calls int32
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		var count int
		if n == 1 {
			count = 250 // full first page
		} else {
			count = 100 // partial second page — signals end of data
		}
		coins := make([]MarketCoin, count)
		for i := range coins {
			coins[i] = MarketCoin{ID: fmt.Sprintf("coin-%d-%d", n, i)}
		}
		_ = json.NewEncoder(w).Encode(coins)
	})
	defer srv.Close()

	// Request 500 but only 350 exist
	result, err := c.FetchAllMarkets(context.Background(), "usd", 500, "market_cap_desc", "")
	require.NoError(t, err)
	assert.Len(t, result, 350)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls)) // stops after partial page
}

func TestFetchAllMarkets_APIErrorMidPagination(t *testing.T) {
	var calls int32
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 2 {
			w.WriteHeader(500)
			_, _ = fmt.Fprint(w, `{"status":{"error_code":500,"error_message":"Internal error"}}`)
			return
		}
		coins := make([]MarketCoin, 250)
		for i := range coins {
			coins[i] = MarketCoin{ID: fmt.Sprintf("coin-%d", i)}
		}
		_ = json.NewEncoder(w).Encode(coins)
	})
	defer srv.Close()

	_, err := c.FetchAllMarkets(context.Background(), "usd", 500, "market_cap_desc", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestFetchAllMarkets_PassesQueryParams(t *testing.T) {
	var gotParams []map[string]string
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotParams = append(gotParams, map[string]string{
			"vs_currency": q.Get("vs_currency"),
			"order":       q.Get("order"),
			"category":    q.Get("category"),
			"per_page":    q.Get("per_page"),
			"page":        q.Get("page"),
		})
		_ = json.NewEncoder(w).Encode([]MarketCoin{})
	})
	defer srv.Close()

	_, _ = c.FetchAllMarkets(context.Background(), "eur", 10, "volume_desc", "layer-1")
	require.Len(t, gotParams, 1)
	assert.Equal(t, "eur", gotParams[0]["vs_currency"])
	assert.Equal(t, "volume_desc", gotParams[0]["order"])
	assert.Equal(t, "layer-1", gotParams[0]["category"])
	assert.Equal(t, "250", gotParams[0]["per_page"])
	assert.Equal(t, "1", gotParams[0]["page"])
}

func TestFetchAllMarkets_EmptyCategory(t *testing.T) {
	c, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		// When category is "", the param should not be set
		assert.Empty(t, r.URL.Query().Get("category"))
		_ = json.NewEncoder(w).Encode([]MarketCoin{})
	})
	defer srv.Close()

	_, err := c.FetchAllMarkets(context.Background(), "usd", 10, "market_cap_desc", "")
	require.NoError(t, err)
}

func TestFetchAllMarkets_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			cancel() // cancel after first request
		}
		coins := make([]MarketCoin, 250)
		for i := range coins {
			coins[i] = MarketCoin{ID: fmt.Sprintf("coin-%d", i)}
		}
		_ = json.NewEncoder(w).Encode(coins)
	}))
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", Tier: config.TierDemo}
	c := NewClient(cfg)
	c.SetBaseURL(srv.URL)

	_, err := c.FetchAllMarkets(ctx, "usd", 500, "market_cap_desc", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
