package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type RuntimeSnapshot struct {
	AppName           string
	ChainID           string
	ProcessedClaims   uint64
	FailedClaims      uint64
	PendingTransfers  int
	TimedOutTransfers int
}

func FormatRuntimeSnapshot(snapshot RuntimeSnapshot) string {
	labels := map[string]string{
		"app_name": snapshot.AppName,
		"chain_id": snapshot.ChainID,
	}

	var builder strings.Builder
	writeMetric(&builder, "aegislink_runtime_processed_claims_total", "counter", "Processed bridge claims.", labels, strconv.FormatUint(snapshot.ProcessedClaims, 10))
	writeMetric(&builder, "aegislink_runtime_failed_claims_total", "counter", "Failed bridge claims.", labels, strconv.FormatUint(snapshot.FailedClaims, 10))
	writeMetric(&builder, "aegislink_runtime_pending_transfers", "gauge", "Pending route transfers.", labels, strconv.Itoa(snapshot.PendingTransfers))
	writeMetric(&builder, "aegislink_runtime_timed_out_transfers_total", "counter", "Timed out route transfers.", labels, strconv.Itoa(snapshot.TimedOutTransfers))
	return builder.String()
}

func writeMetric(builder *strings.Builder, name, metricType, help string, labels map[string]string, value string) {
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
	builder.WriteString(formatLabels(labels))
	builder.WriteByte(' ')
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s=%q`, key, labels[key]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
