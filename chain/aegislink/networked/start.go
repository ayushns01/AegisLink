package networked

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	"google.golang.org/grpc"
)

type ReadyState struct {
	Status      string `json:"status"`
	ChainID     string `json:"chain_id"`
	HomeDir     string `json:"home_dir"`
	RPCAddress  string `json:"rpc_address"`
	GRPCAddress string `json:"grpc_address"`
}

type DemoNode struct {
	appConfig  aegisapp.Config
	app        *aegisapp.App
	config     Config
	httpServer *http.Server
	grpcServer *grpc.Server
	rpcLn      net.Listener
	grpcLn     net.Listener
}

func Start(ctx context.Context, cfg Config) (ReadyState, error) {
	resolved, appCfg, err := ResolveConfig(cfg)
	if err != nil {
		return ReadyState{}, err
	}
	app, err := aegisapp.LoadWithConfig(appCfg)
	if err != nil {
		return ReadyState{}, err
	}

	rpcLn, err := net.Listen("tcp", resolved.RPCAddress)
	if err != nil {
		_ = app.Close()
		return ReadyState{}, err
	}
	grpcLn, err := net.Listen("tcp", resolved.GRPCAddress)
	if err != nil {
		_ = rpcLn.Close()
		_ = app.Close()
		return ReadyState{}, err
	}

	state := ReadyState{
		Status:      "ready",
		ChainID:     appCfg.ChainID,
		HomeDir:     appCfg.HomeDir,
		RPCAddress:  rpcLn.Addr().String(),
		GRPCAddress: grpcLn.Addr().String(),
	}

	node := DemoNode{
		appConfig: appCfg,
		app:       app,
		config:    resolved,
		rpcLn:     rpcLn,
		grpcLn:    grpcLn,
	}
	node.httpServer = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			node.serveHTTP(w, r, state)
		}),
	}
	node.grpcServer = grpc.NewServer()

	if err := writeReadyFile(resolved.ReadyFile, state); err != nil {
		_ = node.Close()
		return ReadyState{}, err
	}

	go func() {
		_ = node.httpServer.Serve(node.rpcLn)
	}()
	go func() {
		node.grpcServer.Serve(node.grpcLn)
	}()
	if resolved.TickInterval > 0 {
		go node.runBlockTicker(ctx, resolved.TickInterval)
	}

	go func() {
		<-ctx.Done()
		_ = node.Close()
	}()

	return state, nil
}

func (n DemoNode) Close() error {
	var errs []error
	if n.httpServer != nil {
		errs = appendCloseErr(errs, n.httpServer.Close())
	}
	if n.grpcServer != nil {
		n.grpcServer.GracefulStop()
	}
	if n.grpcLn != nil {
		errs = appendCloseErr(errs, n.grpcLn.Close())
	}
	if n.app != nil {
		errs = append(errs, n.app.Close())
	}
	return errors.Join(errs...)
}

func (n DemoNode) serveHTTP(w http.ResponseWriter, r *http.Request, ready ReadyState) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/healthz":
		_ = json.NewEncoder(w).Encode(ready)
	case "/status":
		_ = json.NewEncoder(w).Encode(n.app.Status())
	case "/balances":
		address := strings.TrimSpace(r.URL.Query().Get("address"))
		if address == "" {
			http.Error(w, `{"error":"missing address"}`+"\n", http.StatusBadRequest)
			return
		}
		balances, err := n.app.WalletBalances(address)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`+"\n", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(balances)
	case "/transfers":
		transfers := n.app.Transfers()
		sort.Slice(transfers, func(i, j int) bool {
			return transfers[i].TransferID < transfers[j].TransferID
		})
		_ = json.NewEncoder(w).Encode(transferJSONResponseList(transfers))
	case "/tx/queue-deposit-claim":
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`+"\n", http.StatusMethodNotAllowed)
			return
		}
		if err := n.handleQueueDepositClaim(w, r); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`+"\n", http.StatusBadRequest)
		}
	case "/tx/initiate-ibc-transfer":
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`+"\n", http.StatusMethodNotAllowed)
			return
		}
		if err := n.handleInitiateIBCTransfer(w, r); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`+"\n", http.StatusBadRequest)
		}
	default:
		http.NotFound(w, r)
	}
}

func (n DemoNode) runBlockTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = n.advanceBlock()
		}
	}
}

func (n DemoNode) advanceBlock() error {
	n.app.AdvanceBlock()
	return n.app.Save()
}

func (n DemoNode) handleQueueDepositClaim(w http.ResponseWriter, r *http.Request) error {
	claim, attestation, err := decodeSubmission(r)
	if err != nil {
		return err
	}
	if err := n.app.QueueDepositClaim(claim, attestation); err != nil {
		return err
	}
	if err := n.app.Save(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(map[string]any{
		"status":     "queued",
		"message_id": claim.Identity.MessageID,
		"asset_id":   claim.AssetID,
		"amount":     claim.Amount.String(),
	})
}

func (n DemoNode) handleInitiateIBCTransfer(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		RouteID       string `json:"route_id"`
		AssetID       string `json:"asset_id"`
		Amount        string `json:"amount"`
		Receiver      string `json:"receiver"`
		TimeoutHeight uint64 `json:"timeout_height"`
		Memo          string `json:"memo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return err
	}
	if strings.TrimSpace(payload.AssetID) == "" {
		return fmt.Errorf("missing asset id")
	}
	if strings.TrimSpace(payload.Receiver) == "" {
		return fmt.Errorf("missing receiver")
	}
	if payload.TimeoutHeight == 0 {
		return fmt.Errorf("missing timeout height")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(payload.Amount), 10)
	if !ok {
		return fmt.Errorf("invalid amount %q", payload.Amount)
	}

	var transfer ibcrouterkeeper.TransferRecord
	var err error
	if strings.TrimSpace(payload.RouteID) != "" {
		transfer, err = n.app.InitiateIBCTransferWithProfile(payload.RouteID, payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
	} else {
		transfer, err = n.app.InitiateIBCTransfer(payload.AssetID, amount, payload.Receiver, payload.TimeoutHeight, payload.Memo)
	}
	if err != nil {
		return err
	}
	if err := n.app.Save(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(transferJSONResponse(transfer))
}

func transferJSONResponseList(transfers []ibcrouterkeeper.TransferRecord) []map[string]any {
	items := make([]map[string]any, 0, len(transfers))
	for _, transfer := range transfers {
		items = append(items, transferJSONResponse(transfer))
	}
	return items
}

func transferJSONResponse(transfer ibcrouterkeeper.TransferRecord) map[string]any {
	return map[string]any{
		"transfer_id":          transfer.TransferID,
		"asset_id":             transfer.AssetID,
		"amount":               transfer.Amount.String(),
		"receiver":             transfer.Receiver,
		"destination_chain_id": transfer.DestinationChainID,
		"channel_id":           transfer.ChannelID,
		"destination_denom":    transfer.DestinationDenom,
		"timeout_height":       transfer.TimeoutHeight,
		"memo":                 transfer.Memo,
		"status":               string(transfer.Status),
		"failure_reason":       transfer.FailureReason,
	}
}

func appendCloseErr(errs []error, err error) []error {
	if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EINVAL) {
		return errs
	}
	return append(errs, err)
}

func writeReadyFile(path string, state ReadyState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
