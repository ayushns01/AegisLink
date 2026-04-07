package metrics

import (
	"strings"
	"testing"
)

func TestMetricsFormatRuntimeSnapshot(t *testing.T) {
	t.Parallel()

	output := FormatRuntimeSnapshot(RuntimeSnapshot{
		AppName:           "aegislink",
		ChainID:           "aegislink-devnet-1",
		ProcessedClaims:   12,
		FailedClaims:      3,
		PendingTransfers:  5,
		TimedOutTransfers: 2,
	})

	for _, expected := range []string{
		`aegislink_runtime_processed_claims_total{app_name="aegislink",chain_id="aegislink-devnet-1"} 12`,
		`aegislink_runtime_failed_claims_total{app_name="aegislink",chain_id="aegislink-devnet-1"} 3`,
		`aegislink_runtime_pending_transfers{app_name="aegislink",chain_id="aegislink-devnet-1"} 5`,
		`aegislink_runtime_timed_out_transfers_total{app_name="aegislink",chain_id="aegislink-devnet-1"} 2`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected runtime metrics to contain %q\n%s", expected, output)
		}
	}
}
