package metrics

import (
	"strings"
	"testing"
)

func TestMetricsFormatBridgeRunSnapshot(t *testing.T) {
	t.Parallel()

	output := FormatBridgeRunSnapshot(BridgeRunSnapshot{
		DepositsObserved:      6,
		DepositsSubmitted:     4,
		DepositSubmitAttempts: 5,
		WithdrawalsObserved:   3,
		WithdrawalsReleased:   2,
	})

	for _, expected := range []string{
		"aegislink_bridge_relayer_deposits_observed_total 6",
		"aegislink_bridge_relayer_deposits_submitted_total 4",
		"aegislink_bridge_relayer_deposit_submit_attempts_total 5",
		"aegislink_bridge_relayer_withdrawals_observed_total 3",
		"aegislink_bridge_relayer_withdrawals_released_total 2",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected bridge metrics to contain %q\n%s", expected, output)
		}
	}
}

func TestMetricsFormatRouteRunSnapshot(t *testing.T) {
	t.Parallel()

	output := FormatRouteRunSnapshot(RouteRunSnapshot{
		PendingTransfers:   7,
		TransfersDelivered: 3,
		TimedOutAcks:       2,
		FailedAcks:         1,
	})

	for _, expected := range []string{
		"aegislink_route_relayer_pending_transfers 7",
		"aegislink_route_relayer_transfers_delivered_total 3",
		"aegislink_route_relayer_timed_out_acks_total 2",
		"aegislink_route_relayer_failed_acks_total 1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected route metrics to contain %q\n%s", expected, output)
		}
	}
}

func TestMetricsFormatTargetSnapshot(t *testing.T) {
	t.Parallel()

	output := FormatTargetSnapshot(TargetSnapshot{
		Packets:      9,
		Executions:   4,
		SwapFailures: 2,
		ReadyAcks:    3,
	})

	for _, expected := range []string{
		"aegislink_destination_packets 9",
		"aegislink_destination_executions 4",
		"aegislink_destination_swap_failures_total 2",
		"aegislink_destination_ready_acks 3",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected target metrics to contain %q\n%s", expected, output)
		}
	}
}
