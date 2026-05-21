package api

import "encoding/json"

// Simple price response: map[coinID]map[field]value
// Fields include currency price (float64) and 24h change (float64).
type PriceResponse map[string]map[string]float64

type MarketCoin struct {
	ID                       string  `json:"id"`
	Symbol                   string  `json:"symbol"`
	Name                     string  `json:"name"`
	CurrentPrice             float64 `json:"current_price"`
	MarketCap                float64 `json:"market_cap"`
	MarketCapRank            int     `json:"market_cap_rank"`
	TotalVolume              float64 `json:"total_volume"`
	PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
	High24h                  float64 `json:"high_24h"`
	Low24h                   float64 `json:"low_24h"`
	ATH                      float64 `json:"ath"`
	ATHChangePercentage      float64 `json:"ath_change_percentage"`
	ATL                      float64 `json:"atl"`
	ATLChangePercentage      float64 `json:"atl_change_percentage"`
	CirculatingSupply        float64 `json:"circulating_supply"`
	TotalSupply              float64 `json:"total_supply"`
}

type SearchResponse struct {
	Coins []SearchCoin `json:"coins"`
}

type SearchCoin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	MarketCapRank int    `json:"market_cap_rank"`
}

type TrendingResponse struct {
	Coins      []TrendingCoinWrapper `json:"coins"`
	NFTs       []TrendingNFT         `json:"nfts"`
	Categories []TrendingCategory    `json:"categories"`
}

type TrendingCoinWrapper struct {
	Item TrendingCoin `json:"item"`
}

type TrendingCoin struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Symbol        string            `json:"symbol"`
	MarketCapRank int               `json:"market_cap_rank"`
	Score         int               `json:"score"`
	Data          *TrendingCoinData `json:"data"`
}

type TrendingCoinData struct {
	Price                    float64            `json:"price"`
	PriceChangePercentage24h map[string]float64 `json:"price_change_percentage_24h"`
}

type TrendingNFT struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Symbol               string  `json:"symbol"`
	FloorPriceInUSD24hPC float64 `json:"floor_price_24h_percentage_change"`
}

type TrendingCategory struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	MarketCap1hChange float64 `json:"market_cap_1h_change"`
}

type HistoricalData struct {
	ID         string            `json:"id"`
	Symbol     string            `json:"symbol"`
	Name       string            `json:"name"`
	MarketData *HistoricalMarket `json:"market_data"`
}

type HistoricalMarket struct {
	CurrentPrice map[string]float64 `json:"current_price"`
	MarketCap    map[string]float64 `json:"market_cap"`
	TotalVolume  map[string]float64 `json:"total_volume"`
}

// OHLC data: each entry is [timestamp, open, high, low, close]
type OHLCData [][]float64

type MarketChartResponse struct {
	Prices       [][]float64 `json:"prices"`
	MarketCaps   [][]float64 `json:"market_caps"`
	TotalVolumes [][]float64 `json:"total_volumes"`
}

type GainersLosersResponse struct {
	TopGainers []GainerCoin `json:"top_gainers"`
	TopLosers  []GainerCoin `json:"top_losers"`
}

// GainerCoin uses dynamic JSON keys for price fields based on the vs_currency
// parameter. The API returns {currency} and {currency}_24h_change as keys
// (e.g. "usd", "usd_24h_change" or "eur", "eur_24h_change").
type GainerCoin struct {
	ID            string                 `json:"id"`
	Symbol        string                 `json:"symbol"`
	Name          string                 `json:"name"`
	Image         string                 `json:"image"`
	MarketCapRank int                    `json:"market_cap_rank"`
	Extra         map[string]interface{} `json:"-"`
}

// Price returns the price in the given vs currency.
func (g *GainerCoin) Price(vs string) float64 {
	v, _ := g.Extra[vs].(float64)
	return v
}

// PriceChange returns the 24h price change percentage in the given vs currency.
func (g *GainerCoin) PriceChange(vs string) float64 {
	v, _ := g.Extra[vs+"_24h_change"].(float64)
	return v
}

func (g *GainerCoin) UnmarshalJSON(data []byte) error {
	// Single-pass: unmarshal into flat map, then extract known fields.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["id"].(string); ok {
		g.ID = v
	}
	if v, ok := raw["symbol"].(string); ok {
		g.Symbol = v
	}
	if v, ok := raw["name"].(string); ok {
		g.Name = v
	}
	if v, ok := raw["image"].(string); ok {
		g.Image = v
	}
	if v, ok := raw["market_cap_rank"].(float64); ok {
		g.MarketCapRank = int(v)
	}
	g.Extra = raw
	return nil
}

func (g GainerCoin) MarshalJSON() ([]byte, error) {
	// Re-serialize by merging Extra (which has all original fields) back out.
	if g.Extra == nil {
		type Alias GainerCoin
		return json.Marshal(Alias(g))
	}
	return json.Marshal(g.Extra)
}

// TokenPriceResponse: map[contractAddress]map[field]value
// Fields: {vs} (price), {vs}_market_cap, {vs}_24h_vol, {vs}_24h_change, last_updated_at
// https://docs.coingecko.com/reference/simple-token-price
type TokenPriceResponse map[string]map[string]float64

// OnchainTokenPriceResponse is the GeckoTerminal simple token price response.
// All numeric values are strings; callers must use strconv.ParseFloat.
// https://docs.coingecko.com/reference/onchain-simple-price
type OnchainTokenPriceResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			TokenPrices        map[string]string `json:"token_prices"`
			MarketCapUSD       map[string]string `json:"market_cap_usd"`
			H24VolumeUSD       map[string]string `json:"h24_volume_usd"`
			H24PriceChangePct  map[string]string `json:"h24_price_change_percentage"`
			TotalReserveInUSD  map[string]string `json:"total_reserve_in_usd"`
		} `json:"attributes"`
	} `json:"data"`
}

// ExchangeRatesResponse contains BTC-based exchange rates for all supported currencies.
// https://docs.coingecko.com/reference/exchange-rates
type ExchangeRatesResponse struct {
	Rates map[string]ExchangeRate `json:"rates"`
}

// ExchangeRate represents a single currency rate relative to BTC.
type ExchangeRate struct {
	Name  string  `json:"name"`
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

// OnchainSearchPoolsResponse is the response from /onchain/search/pools.
// Used for smart routing: resolving a contract address to its network.
// https://docs.coingecko.com/reference/search-pools
type OnchainSearchPoolsResponse struct {
	Data []OnchainPool `json:"data"`
}

type OnchainPool struct {
	ID            string                `json:"id"`
	Type          string                `json:"type"`
	Relationships OnchainPoolRelations  `json:"relationships"`
}

type OnchainPoolRelations struct {
	BaseToken  OnchainRelRef `json:"base_token"`
	QuoteToken OnchainRelRef `json:"quote_token"`
}

type OnchainRelRef struct {
	Data struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"data"`
}

// OnchainNetworksResponse is the response from /onchain/networks.
// https://docs.coingecko.com/reference/networks-list
type OnchainNetworksResponse struct {
	Data []OnchainNetwork `json:"data"`
}

type OnchainNetwork struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name                     string `json:"name"`
		CoingeckoAssetPlatformID string `json:"coingecko_asset_platform_id"`
	} `json:"attributes"`
}

type CoinDetail struct {
	ID            string `json:"id"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	MarketCapRank int    `json:"market_cap_rank"`
	Description   struct {
		EN string `json:"en"`
	} `json:"description"`
	MarketData *CoinDetailMarket `json:"market_data"`
}

type CoinDetailMarket struct {
	CurrentPrice             map[string]float64 `json:"current_price"`
	MarketCap                map[string]float64 `json:"market_cap"`
	TotalVolume              map[string]float64 `json:"total_volume"`
	High24h                  map[string]float64 `json:"high_24h"`
	Low24h                   map[string]float64 `json:"low_24h"`
	PriceChangePercentage24h float64            `json:"price_change_percentage_24h"`
	ATH                      map[string]float64 `json:"ath"`
	ATHChangePercentage      map[string]float64 `json:"ath_change_percentage"`
	ATHDate                  map[string]string  `json:"ath_date"`
	ATL                      map[string]float64 `json:"atl"`
	ATLChangePercentage      map[string]float64 `json:"atl_change_percentage"`
	ATLDate                  map[string]string  `json:"atl_date"`
	CirculatingSupply        float64            `json:"circulating_supply"`
	TotalSupply              float64            `json:"total_supply"`
}
