package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMonitoringAssetsIncludePrometheusAndGrafanaSurfaces(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	prometheusConfigPath := filepath.Join(repoRoot, "deploy", "monitoring", "prometheus.yml")
	prometheusConfig, err := os.ReadFile(prometheusConfigPath)
	if err != nil {
		t.Fatalf("read prometheus config: %v", err)
	}
	for _, expected := range []string{
		"job_name: mock-osmosis-target",
		"metrics_path: /metrics",
		"mock-osmosis-target:9191",
	} {
		if !strings.Contains(string(prometheusConfig), expected) {
			t.Fatalf("expected prometheus config to contain %q\n%s", expected, string(prometheusConfig))
		}
	}

	datasourcePath := filepath.Join(repoRoot, "deploy", "monitoring", "grafana", "provisioning", "datasources", "prometheus.yml")
	datasourceConfig, err := os.ReadFile(datasourcePath)
	if err != nil {
		t.Fatalf("read grafana datasource config: %v", err)
	}
	if !strings.Contains(string(datasourceConfig), "url: http://prometheus:9090") {
		t.Fatalf("expected grafana datasource to point at prometheus, got:\n%s", string(datasourceConfig))
	}

	dashboardPath := filepath.Join(repoRoot, "deploy", "monitoring", "grafana", "dashboards", "aegislink-overview.json")
	dashboardJSON, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read grafana dashboard: %v", err)
	}
	var dashboard map[string]any
	if err := json.Unmarshal(dashboardJSON, &dashboard); err != nil {
		t.Fatalf("decode grafana dashboard: %v", err)
	}
	panels, ok := dashboard["panels"].([]any)
	if !ok || len(panels) == 0 {
		t.Fatalf("expected dashboard panels, got %+v", dashboard["panels"])
	}
	serialized := string(dashboardJSON)
	for _, expected := range []string{
		"aegislink_destination_packets",
		"aegislink_destination_executions",
		"aegislink_destination_swap_failures_total",
		"aegislink_destination_ready_acks",
	} {
		if !strings.Contains(serialized, expected) {
			t.Fatalf("expected dashboard to reference %q\n%s", expected, serialized)
		}
	}
}
