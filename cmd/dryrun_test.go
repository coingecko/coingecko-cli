package cmd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDryRun_OperationIDFallback verifies that when a command defines
// per-mode OASOperationIDs but the requested opKey is NOT in the map,
// the output falls back to the command-level default OASOperationID.
func TestDryRun_OperationIDFallback(t *testing.T) {
	const cmdName = "_test_opid_fallback"
	commandMeta[cmdName] = commandAnnotation{
		OASSpec:        "default-spec.json",
		OASOperationID: "default-op",
		OASOperationIDs: map[string]string{
			"--modeA": "modeA-op",
		},
		RequiresAuth: true,
	}
	t.Cleanup(func() { delete(commandMeta, cmdName) })

	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("dry-run should not make HTTP calls")
	})
	defer srv.Close()
	withTestClientDemo(t, srv)

	tests := []struct {
		name            string
		opKey           string
		wantOperationID string
	}{
		{
			name:            "opKey present in map",
			opKey:           "--modeA",
			wantOperationID: "modeA-op",
		},
		{
			name:            "opKey missing falls back to default",
			opKey:           "--modeB",
			wantOperationID: "default-op",
		},
		{
			name:            "empty opKey uses default",
			opKey:           "",
			wantOperationID: "default-op",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := captureStdout(t, func() {
				cfg, err := loadConfig()
				require.NoError(t, err)
				err = printDryRunFull(cfg, cmdName, tt.opKey, "/test/endpoint", nil, nil, "")
				require.NoError(t, err)
			})

			var out dryRunOutput
			require.NoError(t, json.Unmarshal([]byte(stdout), &out))
			assert.Equal(t, "default-spec.json", out.OASSpec)
			assert.Equal(t, tt.wantOperationID, out.OASOperationID)
		})
	}
}
