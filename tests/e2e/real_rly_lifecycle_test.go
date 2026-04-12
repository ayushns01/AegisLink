package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ayushns01/aegislink/chain/aegislink/networked"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRealRlyLinksTwoDemoNodes(t *testing.T) {
	if os.Getenv("AEGISLINK_ENABLE_REAL_RLY") != "1" {
		t.Skip("set AEGISLINK_ENABLE_REAL_RLY=1 to run the live local relayer lifecycle test")
	}

	relayerBin := filepath.Join(repoRoot(t), "bin", "relayer")
	if _, err := os.Stat(relayerBin); err != nil {
		t.Skipf("local relayer binary not available at %s: %v", relayerBin, err)
	}

	tempDir := t.TempDir()
	srcHome := filepath.Join(tempDir, "src-home")
	dstHome := filepath.Join(tempDir, "dst-home")
	srcReadyPath := filepath.Join(tempDir, "src-ready.json")
	dstReadyPath := filepath.Join(tempDir, "dst-ready.json")
	rlyHome := filepath.Join(tempDir, "rly-home")

	bootstrapPublicAegisLinkTestnetWithChainID(t, srcHome, "aegislink-src-1")
	bootstrapPublicAegisLinkTestnetWithChainID(t, dstHome, "aegislink-dst-1")

	srcCmd, srcLogs := startIBCDemoNodeProcess(t, srcHome, srcReadyPath, map[string]string{
		"AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS": "10",
	})
	defer stopIBCDemoNodeProcess(t, srcCmd, srcLogs)
	dstCmd, dstLogs := startIBCDemoNodeProcess(t, dstHome, dstReadyPath, map[string]string{
		"AEGISLINK_DEMO_NODE_TICK_INTERVAL_MS": "10",
	})
	defer stopIBCDemoNodeProcess(t, dstCmd, dstLogs)

	srcReady := readReadyFileE2E(t, srcReadyPath)
	dstReady := readReadyFileE2E(t, dstReadyPath)

	writeRealRlyConfig(t, filepath.Join(rlyHome, "config", "config.yaml"), realRlyConfig{
		SourceChainID: "aegislink-src-1",
		SourceRPC:     "http://" + srcReady.CometRPCAddress,
		SourceRPCWS:   "ws://" + srcReady.CometRPCAddress + "/websocket",
		SourceGRPC:    "http://" + srcReady.GRPCAddress,
		DestChainID:   "aegislink-dst-1",
		DestRPC:       "http://" + dstReady.CometRPCAddress,
		DestRPCWS:     "ws://" + dstReady.CometRPCAddress + "/websocket",
		DestGRPC:      "http://" + dstReady.GRPCAddress,
	})

	runRelayerBinary(t, relayerBin, rlyHome, nil, "keys", "add", "aegislink-src-1", "srckey")
	runRelayerBinary(t, relayerBin, rlyHome, nil, "keys", "add", "aegislink-dst-1", "dstkey")

	srcRelayerAddr := strings.TrimSpace(runRelayerBinary(t, relayerBin, rlyHome, nil, "keys", "show", "aegislink-src-1", "srckey"))
	dstRelayerAddr := strings.TrimSpace(runRelayerBinary(t, relayerBin, rlyHome, nil, "keys", "show", "aegislink-dst-1", "dstkey"))

	if _, err := networked.SubmitFundAccount(context.Background(), networked.Config{
		HomeDir:   srcHome,
		ReadyFile: srcReadyPath,
	}, srcRelayerAddr, "stake", "1000000000"); err != nil {
		t.Fatalf("fund source relayer account: %v", err)
	}
	if _, err := networked.SubmitFundAccount(context.Background(), networked.Config{
		HomeDir:   dstHome,
		ReadyFile: dstReadyPath,
	}, dstRelayerAddr, "stake", "1000000000"); err != nil {
		t.Fatalf("fund destination relayer account: %v", err)
	}

	runRelayerBinary(t, relayerBin, rlyHome, nil, "paths", "new", "aegislink-src-1", "aegislink-dst-1", "demo")
	runRelayerBinary(t, relayerBin, rlyHome, []*bytes.Buffer{srcLogs, dstLogs}, "transact", "link", "demo", "--debug", "--log-level", "debug")

	assertIBCHandshakeState(t, srcReady.GRPCAddress)
	assertIBCHandshakeState(t, dstReady.GRPCAddress)
}

type realRlyConfig struct {
	SourceChainID string
	SourceRPC     string
	SourceRPCWS   string
	SourceGRPC    string
	DestChainID   string
	DestRPC       string
	DestRPCWS     string
	DestGRPC      string
}

func bootstrapPublicAegisLinkTestnetWithChainID(t *testing.T, homeDir, chainID string) {
	t.Helper()

	cmd := exec.Command("bash", "scripts/testnet/bootstrap_aegislink_testnet.sh", homeDir)
	cmd.Dir = repoRoot(t)
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
		"AEGISLINK_PUBLIC_CHAIN_ID="+chainID,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap public aegislink testnet %s: %v\n%s", chainID, err, output)
	}
}

func writeRealRlyConfig(t *testing.T, path string, cfg realRlyConfig) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir real rly config dir: %v", err)
	}

	body := fmt.Sprintf(`global:
  api-listen-addr: :5183
  timeout: 20s
  memo: ""
chains:
  %s:
    type: cosmos
    value:
      key: srckey
      chain-id: %s
      rpc-addr: %s
      websocket-addr: %s
      grpc-addr: %s
      account-prefix: cosmos
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: 0stake
      debug: true
      timeout: 20s
      output-format: json
      sign-mode: direct
  %s:
    type: cosmos
    value:
      key: dstkey
      chain-id: %s
      rpc-addr: %s
      websocket-addr: %s
      grpc-addr: %s
      account-prefix: cosmos
      keyring-backend: test
      gas-adjustment: 1.3
      gas-prices: 0stake
      debug: true
      timeout: 20s
      output-format: json
      sign-mode: direct
paths: {}
`, cfg.SourceChainID, cfg.SourceChainID, cfg.SourceRPC, cfg.SourceRPCWS, cfg.SourceGRPC, cfg.DestChainID, cfg.DestChainID, cfg.DestRPC, cfg.DestRPCWS, cfg.DestGRPC)

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write real rly config: %v", err)
	}
}

func runRelayerBinary(t *testing.T, relayerBin, home string, extraLogs []*bytes.Buffer, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, relayerBin, append(args, "--home", home)...)
	cmd.Dir = repoRoot(t)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("relayer command timed out: %s %s\n%s%s", relayerBin, strings.Join(args, " "), output, formatExtraLogs(extraLogs))
	}
	if err != nil {
		t.Fatalf("relayer command failed: %s %s\n%v\n%s%s", relayerBin, strings.Join(args, " "), err, output, formatExtraLogs(extraLogs))
	}
	return string(output)
}

func formatExtraLogs(buffers []*bytes.Buffer) string {
	if len(buffers) == 0 {
		return ""
	}

	var builder strings.Builder
	labels := []string{"source_node_logs", "destination_node_logs"}
	for i, buffer := range buffers {
		if buffer == nil || buffer.Len() == 0 {
			continue
		}
		label := fmt.Sprintf("extra_logs_%d", i)
		if i < len(labels) {
			label = labels[i]
		}
		builder.WriteString("\n")
		builder.WriteString(label)
		builder.WriteString(":\n")
		builder.WriteString(buffer.String())
	}
	return builder.String()
}

func assertIBCHandshakeState(t *testing.T, grpcAddress string) {
	t.Helper()

	grpcConn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial ibc demo node grpc %s: %v", grpcAddress, err)
	}
	defer grpcConn.Close()

	clientResp, err := clienttypes.NewQueryClient(grpcConn).ClientStates(context.Background(), &clienttypes.QueryClientStatesRequest{})
	if err != nil {
		t.Fatalf("query ibc client states from %s: %v", grpcAddress, err)
	}
	if len(clientResp.ClientStates) == 0 {
		t.Fatalf("expected at least one ibc client state on %s, got %+v", grpcAddress, clientResp)
	}

	connectionResp, err := conntypes.NewQueryClient(grpcConn).Connections(context.Background(), &conntypes.QueryConnectionsRequest{})
	if err != nil {
		t.Fatalf("query ibc connections from %s: %v", grpcAddress, err)
	}
	if len(connectionResp.Connections) == 0 {
		t.Fatalf("expected at least one ibc connection on %s, got %+v", grpcAddress, connectionResp)
	}

	channelResp, err := channeltypes.NewQueryClient(grpcConn).Channels(context.Background(), &channeltypes.QueryChannelsRequest{})
	if err != nil {
		t.Fatalf("query ibc channels from %s: %v", grpcAddress, err)
	}
	if len(channelResp.Channels) == 0 {
		t.Fatalf("expected at least one ibc channel on %s, got %+v", grpcAddress, channelResp)
	}
}
