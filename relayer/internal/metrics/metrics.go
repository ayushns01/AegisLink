package metrics

import (
	"strconv"
	"strings"
)

type BridgeRunSnapshot struct {
	DepositsObserved      int
	DepositsSubmitted     int
	DepositSubmitAttempts int
	WithdrawalsObserved   int
	WithdrawalsReleased   int
}

type RouteRunSnapshot struct {
	PendingTransfers   int
	TransfersDelivered int
	TimedOutAcks       int
	FailedAcks         int
}

type TargetSnapshot struct {
	Packets      int
	Executions   int
	SwapFailures int
	ReadyAcks    int
}

func FormatBridgeRunSnapshot(snapshot BridgeRunSnapshot) string {
	var builder strings.Builder
	writeMetric(&builder, "aegislink_bridge_relayer_deposits_observed_total", "counter", "Observed deposit events.", itoa(snapshot.DepositsObserved))
	writeMetric(&builder, "aegislink_bridge_relayer_deposits_submitted_total", "counter", "Submitted deposit claims.", itoa(snapshot.DepositsSubmitted))
	writeMetric(&builder, "aegislink_bridge_relayer_deposit_submit_attempts_total", "counter", "Deposit submit attempts.", itoa(snapshot.DepositSubmitAttempts))
	writeMetric(&builder, "aegislink_bridge_relayer_withdrawals_observed_total", "counter", "Observed withdrawals.", itoa(snapshot.WithdrawalsObserved))
	writeMetric(&builder, "aegislink_bridge_relayer_withdrawals_released_total", "counter", "Released withdrawals.", itoa(snapshot.WithdrawalsReleased))
	return builder.String()
}

func FormatRouteRunSnapshot(snapshot RouteRunSnapshot) string {
	var builder strings.Builder
	writeMetric(&builder, "aegislink_route_relayer_pending_transfers", "gauge", "Pending route transfers.", itoa(snapshot.PendingTransfers))
	writeMetric(&builder, "aegislink_route_relayer_transfers_delivered_total", "counter", "Delivered route transfers.", itoa(snapshot.TransfersDelivered))
	writeMetric(&builder, "aegislink_route_relayer_timed_out_acks_total", "counter", "Timed out acknowledgements.", itoa(snapshot.TimedOutAcks))
	writeMetric(&builder, "aegislink_route_relayer_failed_acks_total", "counter", "Failed acknowledgements.", itoa(snapshot.FailedAcks))
	return builder.String()
}

func FormatTargetSnapshot(snapshot TargetSnapshot) string {
	var builder strings.Builder
	writeMetric(&builder, "aegislink_destination_packets", "gauge", "Destination packets.", itoa(snapshot.Packets))
	writeMetric(&builder, "aegislink_destination_executions", "gauge", "Destination executions.", itoa(snapshot.Executions))
	writeMetric(&builder, "aegislink_destination_swap_failures_total", "counter", "Destination swap failures.", itoa(snapshot.SwapFailures))
	writeMetric(&builder, "aegislink_destination_ready_acks", "gauge", "Ready destination acknowledgements.", itoa(snapshot.ReadyAcks))
	return builder.String()
}

func writeMetric(builder *strings.Builder, name, metricType, help, value string) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteString(" ")
	builder.WriteString(help)
	builder.WriteByte('\n')
	builder.WriteString("# TYPE ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(metricType)
	builder.WriteByte('\n')
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
