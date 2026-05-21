package cmd

import (
	"github.com/coingecko/coingecko-cli/internal/config"
	"github.com/coingecko/coingecko-cli/internal/ws"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().Bool("dry-run", false, "Preview API request without executing (JSON output)")
}

func isDryRun(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("dry-run")
	return v
}

type dryRunOutput struct {
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Params         map[string]string `json:"params"`
	Headers        map[string]string `json:"headers"`
	OASOperationID string            `json:"oas_operation_id,omitempty"`
	OASSpec        string            `json:"oas_spec,omitempty"`
	Note           string            `json:"note,omitempty"`
	Pagination     *paginationInfo   `json:"pagination"`
}

type paginationInfo struct {
	TotalRequested int `json:"total_requested"`
	PerPage        int `json:"per_page"`
	Pages          int `json:"pages"`
}

func printDryRun(cfg *config.Config, cmdName, endpoint string, params map[string]string, pagination *paginationInfo) error {
	return printDryRunWithOp(cfg, cmdName, "", endpoint, params, pagination)
}

func printDryRunWithOp(cfg *config.Config, cmdName, opKey, endpoint string, params map[string]string, pagination *paginationInfo) error {
	return printDryRunFull(cfg, cmdName, opKey, endpoint, params, pagination, "")
}

type dryRunWSOutput struct {
	PreflightRequests []dryRunOutput `json:"preflight_requests,omitempty"`
	Transport         string         `json:"transport"`
	URL               string         `json:"url"`
	SubscribePayload  any            `json:"subscribe_payload"`
	SetTokensPayload  any            `json:"set_tokens_payload"`
	Note              string         `json:"note,omitempty"`
}

func printDryRunWS(cfg *config.Config, coinIDs []string, preflights []dryRunOutput) error {
	masked := cfg.MaskedKey()
	url := ws.DefaultWSURL + "?x_cg_pro_api_key=" + masked

	out := dryRunWSOutput{
		PreflightRequests: preflights,
		Transport:         "websocket",
		URL:               url,
		SubscribePayload: map[string]string{
			"command":    "subscribe",
			"identifier": ws.ChannelID,
		},
		SetTokensPayload: map[string]any{
			"command":    "message",
			"identifier": ws.ChannelID,
			"data":       map[string]any{"action": "set_tokens", "coin_id": coinIDs},
		},
		Note: "Paid plan required. Updates stream as NDJSON (~every 10s). USD prices only.",
	}

	return printJSONRaw(out)
}

func printDryRunFull(cfg *config.Config, cmdName, opKey, endpoint string, params map[string]string, pagination *paginationInfo, note string) error {
	headerKey, _ := cfg.AuthHeader()
	masked := cfg.MaskedKey()

	headers := map[string]string{}
	if cfg.APIKey != "" {
		headers[headerKey] = masked
	}
	headers["Accept"] = "application/json"
	headers["User-Agent"] = userAgent

	out := dryRunOutput{
		Method:     "GET",
		URL:        cfg.BaseURL() + endpoint,
		Params:     params,
		Headers:    headers,
		Note:       note,
		Pagination: pagination,
	}

	if meta, ok := commandMeta[cmdName]; ok {
		out.OASSpec = meta.OASSpec
		out.OASOperationID = meta.OASOperationID
		if opKey != "" && meta.OASOperationIDs != nil {
			if v, ok := meta.OASOperationIDs[opKey]; ok {
				out.OASOperationID = v
			}
		}
	}

	return printJSONRaw(out)
}
