package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// SimplePrice fetches current prices for the given coin IDs.
// https://docs.coingecko.com/v3.0.1/reference/simple-price
func (c *Client) SimplePrice(ctx context.Context, ids []string, vsCurrency string) (PriceResponse, error) {
	params := url.Values{
		"ids":                 {strings.Join(ids, ",")},
		"vs_currencies":       {vsCurrency},
		"include_24hr_change": {"true"},
	}
	var result PriceResponse
	err := c.get(ctx, "/simple/price?"+params.Encode(), &result)
	return result, err
}

// SimplePriceBySymbols fetches current prices by ticker symbols (e.g. btc, eth).
// https://docs.coingecko.com/v3.0.1/reference/simple-price
func (c *Client) SimplePriceBySymbols(ctx context.Context, symbols []string, vsCurrency string) (PriceResponse, error) {
	params := url.Values{
		"symbols":              {strings.Join(symbols, ",")},
		"vs_currencies":        {vsCurrency},
		"include_24hr_change":  {"true"},
	}
	var result PriceResponse
	err := c.get(ctx, "/simple/price?"+params.Encode(), &result)
	return result, err
}

// SimpleTokenPrice fetches current prices for tokens by contract address on a given platform.
// https://docs.coingecko.com/v3.0.1/reference/simple-token-price
func (c *Client) SimpleTokenPrice(ctx context.Context, platform string, contractAddresses []string, vsCurrency string) (PriceResponse, error) {
	params := url.Values{
		"contract_addresses": {strings.Join(contractAddresses, ",")},
		"vs_currencies":      {vsCurrency},
		"include_24hr_change": {"true"},
	}
	var result PriceResponse
	err := c.get(ctx, fmt.Sprintf("/simple/token_price/%s?%s", url.PathEscape(platform), params.Encode()), &result)
	return result, err
}

// CoinMarkets fetches a paginated list of coins with market data.
// https://docs.coingecko.com/v3.0.1/reference/coins-markets
func (c *Client) CoinMarkets(ctx context.Context, vsCurrency string, perPage, page int, order, category string) ([]MarketCoin, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"per_page":    {fmt.Sprintf("%d", perPage)},
		"page":        {fmt.Sprintf("%d", page)},
		"order":       {order},
	}
	if category != "" {
		params.Set("category", category)
	}
	var result []MarketCoin
	err := c.get(ctx, "/coins/markets?"+params.Encode(), &result)
	return result, err
}

// FetchAllMarkets fetches up to total coins with automatic pagination (250 per page).
func (c *Client) FetchAllMarkets(ctx context.Context, vsCurrency string, total int, order, category string) ([]MarketCoin, error) {
	const perPage = 250
	initCap := total
	if initCap > perPage {
		initCap = perPage
	}
	allCoins := make([]MarketCoin, 0, initCap)
	for page := 1; len(allCoins) < total; page++ {
		coins, err := c.CoinMarkets(ctx, vsCurrency, perPage, page, order, category)
		if err != nil {
			return nil, err
		}
		allCoins = append(allCoins, coins...)
		if len(coins) < perPage {
			break
		}
	}
	if len(allCoins) > total {
		allCoins = allCoins[:total]
	}
	return allCoins, nil
}

// Search queries the CoinGecko search endpoint.
// https://docs.coingecko.com/v3.0.1/reference/search-data
func (c *Client) Search(ctx context.Context, query string) (*SearchResponse, error) {
	params := url.Values{"query": {query}}
	var result SearchResponse
	err := c.get(ctx, "/search?"+params.Encode(), &result)
	return &result, err
}

// SearchTrending fetches trending coins, NFTs, and categories.
// https://docs.coingecko.com/v3.0.1/reference/trending-search
func (c *Client) SearchTrending(ctx context.Context, showMax string) (*TrendingResponse, error) {
	path := "/search/trending"
	if showMax != "" {
		params := url.Values{"show_max": {showMax}}
		path += "?" + params.Encode()
	}
	var result TrendingResponse
	err := c.get(ctx, path, &result)
	return &result, err
}

// CoinHistory fetches historical data for a coin on a specific date (DD-MM-YYYY).
// https://docs.coingecko.com/v3.0.1/reference/coins-id-history
func (c *Client) CoinHistory(ctx context.Context, id, date string) (*HistoricalData, error) {
	params := url.Values{"date": {date}, "localization": {"false"}}
	var result HistoricalData
	err := c.get(ctx, fmt.Sprintf("/coins/%s/history?%s", url.PathEscape(id), params.Encode()), &result)
	return &result, err
}

// CoinMarketChart fetches price/market data for the last N days.
// Paid plans support interval param: 5m (Enterprise), hourly, daily.
// https://docs.coingecko.com/reference/coins-id-market-chart
func (c *Client) CoinMarketChart(ctx context.Context, id, vsCurrency, days, interval string) (*MarketChartResponse, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"days":        {days},
	}
	if interval != "" {
		params.Set("interval", interval)
	}
	var result MarketChartResponse
	err := c.get(ctx, fmt.Sprintf("/coins/%s/market_chart?%s", url.PathEscape(id), params.Encode()), &result)
	return &result, err
}

// CoinMarketChartRange fetches price data for a date range (UNIX timestamps in seconds).
// Paid plans support interval param: 5m (Enterprise), hourly, daily.
// https://docs.coingecko.com/reference/coins-id-market-chart-range
func (c *Client) CoinMarketChartRange(ctx context.Context, id, vsCurrency string, from, to int64, interval string) (*MarketChartResponse, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"from":        {fmt.Sprintf("%d", from)},
		"to":          {fmt.Sprintf("%d", to)},
	}
	if interval != "" {
		params.Set("interval", interval)
	}
	var result MarketChartResponse
	err := c.get(ctx, fmt.Sprintf("/coins/%s/market_chart/range?%s", url.PathEscape(id), params.Encode()), &result)
	return &result, err
}

// CoinOHLC fetches OHLC data for the last N days.
// Valid days: 1, 7, 14, 30, 90, 180, 365, max (paid).
// Paid plans support interval param: daily, hourly.
// https://docs.coingecko.com/reference/coins-id-ohlc
func (c *Client) CoinOHLC(ctx context.Context, id, vsCurrency, days, interval string) (OHLCData, error) {
	params := url.Values{
		"vs_currency": {vsCurrency},
		"days":        {days},
	}
	if interval != "" {
		params.Set("interval", interval)
	}
	var result OHLCData
	err := c.get(ctx, fmt.Sprintf("/coins/%s/ohlc?%s", url.PathEscape(id), params.Encode()), &result)
	return result, err
}

// CoinOHLCRange fetches OHLC data for a date range (UNIX timestamps in seconds, paid plans only).
// https://docs.coingecko.com/reference/coins-id-ohlc-range
func (c *Client) CoinOHLCRange(ctx context.Context, id, vsCurrency string, from, to int64, interval string) (OHLCData, error) {
	if err := c.requirePaid(); err != nil {
		return nil, err
	}
	params := url.Values{
		"vs_currency": {vsCurrency},
		"from":        {fmt.Sprintf("%d", from)},
		"to":          {fmt.Sprintf("%d", to)},
	}
	if interval != "" {
		params.Set("interval", interval)
	}
	var result OHLCData
	err := c.get(ctx, fmt.Sprintf("/coins/%s/ohlc/range?%s", url.PathEscape(id), params.Encode()), &result)
	return result, err
}

// TopGainersLosers fetches top gaining and losing coins (paid plans only).
// https://docs.coingecko.com/reference/coins-top-gainers-losers
func (c *Client) TopGainersLosers(ctx context.Context, vsCurrency, duration, topCoins, priceChangePct string) (*GainersLosersResponse, error) {
	if err := c.requirePaid(); err != nil {
		return nil, err
	}
	params := url.Values{
		"vs_currency": {vsCurrency},
		"duration":    {duration},
		"top_coins":   {topCoins},
	}
	if priceChangePct != "" {
		params.Set("price_change_percentage", priceChangePct)
	}
	var result GainersLosersResponse
	err := c.get(ctx, "/coins/top_gainers_losers?"+params.Encode(), &result)
	return &result, err
}

// CoinDetail fetches detailed coin data (used in TUI detail view).
// https://docs.coingecko.com/v3.0.1/reference/coins-id
func (c *Client) CoinDetail(ctx context.Context, id string) (*CoinDetail, error) {
	params := url.Values{
		"localization":   {"false"},
		"tickers":        {"false"},
		"community_data": {"false"},
		"developer_data": {"false"},
	}
	var result CoinDetail
	err := c.get(ctx, fmt.Sprintf("/coins/%s?%s", url.PathEscape(id), params.Encode()), &result)
	return &result, err
}
