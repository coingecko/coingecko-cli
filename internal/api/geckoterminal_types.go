package api

// GTTokenResponse is the GeckoTerminal /networks/{network}/tokens/{address} response.
type GTTokenResponse struct {
	Data GTTokenData `json:"data"`
}

type GTTokenData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes GTTokenAttrs   `json:"attributes"`
}

type GTTokenAttrs struct {
	Name            string  `json:"name"`
	Symbol          string  `json:"symbol"`
	Address         string  `json:"address"`
	Decimals        int     `json:"decimals"`
	CoingeckoCoinID string  `json:"coingecko_coin_id"`
	PriceUSD        string  `json:"price_usd"`
	FDVInUSD        string  `json:"fdv_usd"`
	TotalSupply     string  `json:"total_supply"`
	Volume24h       string  `json:"volume_usd"`
	MarketCapUSD    string  `json:"market_cap_usd"`
	GTScore         float64 `json:"gt_score"`
	ImageURL        string  `json:"image_url"`
}

// GTPoolsResponse is the GeckoTerminal /networks/{network}/tokens/{address}/pools response.
type GTPoolsResponse struct {
	Data []GTPoolData `json:"data"`
}

type GTPoolData struct {
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Attributes GTPoolAttrs `json:"attributes"`
}

type GTPoolAttrs struct {
	Name              string  `json:"name"`
	Address           string  `json:"address"`
	BaseTokenPriceUSD string  `json:"base_token_price_usd"`
	QuoteTokenPriceUSD string `json:"quote_token_price_usd"`
	PoolCreatedAt     string  `json:"pool_created_at"`
	ReserveInUSD      string  `json:"reserve_in_usd"`
	FDVInUSD          string  `json:"fdv_usd"`
	Volume24h         GTVolumeData `json:"volume_usd"`
	GTScore           float64 `json:"gt_score"`
}

type GTVolumeData struct {
	H24 string `json:"h24"`
}
