package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	geckoTerminalBaseURL = "https://api.geckoterminal.com/api/v2"
)

// GeckoTerminalClient accesses GeckoTerminal's public API.
// GeckoTerminal is owned by CoinGecko and provides on-chain DEX data
// including pool safety scores (gt_score).
type GeckoTerminalClient struct {
	http      *http.Client
	baseURL   string
	UserAgent string
}

// NewGeckoTerminalClient creates a client for the GeckoTerminal API.
func NewGeckoTerminalClient() *GeckoTerminalClient {
	return &GeckoTerminalClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: geckoTerminalBaseURL,
	}
}

// SetBaseURL overrides the base URL (used in tests).
func (c *GeckoTerminalClient) SetBaseURL(url string) {
	c.baseURL = url
}

func (c *GeckoTerminalClient) get(ctx context.Context, path string, result any) error {
	reqURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return fmt.Errorf("GeckoTerminal API error %d: %s", resp.StatusCode, string(body))
	}

	lr := io.LimitReader(resp.Body, maxResponseBodySize+1)
	dec := json.NewDecoder(lr)
	if err := dec.Decode(result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	return nil
}

// TokenInfo fetches token data from GeckoTerminal.
// https://www.geckoterminal.com/dex-api
func (c *GeckoTerminalClient) TokenInfo(ctx context.Context, network, address string) (*GTTokenResponse, error) {
	path := fmt.Sprintf("/networks/%s/tokens/%s",
		url.PathEscape(network), url.PathEscape(address))
	var result GTTokenResponse
	err := c.get(ctx, path, &result)
	return &result, err
}

// TokenPools fetches top pools for a token, including gt_score.
// https://www.geckoterminal.com/dex-api
func (c *GeckoTerminalClient) TokenPools(ctx context.Context, network, address string) (*GTPoolsResponse, error) {
	path := fmt.Sprintf("/networks/%s/tokens/%s/pools?sort=h24_volume_usd_liquidity_desc",
		url.PathEscape(network), url.PathEscape(address))
	var result GTPoolsResponse
	err := c.get(ctx, path, &result)
	return &result, err
}
