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
	"strconv"
	"strings"
	"syscall"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	abciserver "github.com/cometbft/cometbft/abci/server"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	cmtservice "github.com/cometbft/cometbft/libs/service"
	cmtnode "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/proxy"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sm "github.com/cometbft/cometbft/state"
	cmtstore "github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
	"google.golang.org/grpc"
)

type ReadyState struct {
	Status                 string   `json:"status"`
	ChainID                string   `json:"chain_id"`
	NodeID                 string   `json:"node_id"`
	HomeDir                string   `json:"home_dir"`
	RPCAddress             string   `json:"rpc_address"`
	CometRPCAddress        string   `json:"comet_rpc_address"`
	GRPCAddress            string   `json:"grpc_address"`
	ABCIAddress            string   `json:"abci_address"`
	ConfigPath             string   `json:"config_path"`
	CometGenesisPath       string   `json:"comet_genesis_path"`
	SDKGenesisPath         string   `json:"sdk_genesis_path"`
	NodeKeyPath            string   `json:"node_key_path"`
	PrivValidatorKeyPath   string   `json:"priv_validator_key_path"`
	PrivValidatorStatePath string   `json:"priv_validator_state_path"`
	CoreStoreKeys          []string `json:"core_store_keys"`
	SDKGenesisModules      []string `json:"sdk_genesis_modules"`
}

type DemoNode struct {
	appConfig  aegisapp.Config
	app        *aegisapp.App
	config     Config
	httpServer *http.Server
	grpcServer *grpc.Server
	abciServer cmtservice.Service
	cometNode  *cmtnode.Node
	rpcLn      net.Listener
	grpcLn     net.Listener
}

func Start(ctx context.Context, cfg Config) (ReadyState, error) {
	resolved, appCfg, err := ResolveConfig(cfg)
	if err != nil {
		return ReadyState{}, err
	}
	resolved.ABCIAddress, err = resolveConcreteTCPAddress(resolved.ABCIAddress)
	if err != nil {
		return ReadyState{}, err
	}
	resolved.CometRPCAddress, err = resolveConcreteTCPAddress(resolved.CometRPCAddress)
	if err != nil {
		return ReadyState{}, err
	}
	resolved.P2PAddress, err = resolveConcreteTCPAddress(resolved.P2PAddress)
	if err != nil {
		return ReadyState{}, err
	}

	chainApp, err := NewChainApp(resolved)
	if err != nil {
		return ReadyState{}, err
	}
	coreStoreKeys := chainApp.SortedStoreKeyNames()
	sdkGenesisModules := sortedGenesisModuleNames(chainApp.DefaultGenesis())
	if err := chainApp.Close(); err != nil {
		return ReadyState{}, err
	}

	nodeHome, err := ensureCometNodeHome(resolved, appCfg)
	if err != nil {
		return ReadyState{}, err
	}
	app, err := aegisapp.LoadWithConfig(appCfg)
	if err != nil {
		return ReadyState{}, err
	}
	abciApp := NewABCIApplication(appCfg, app)
	abciSvc, err := abciserver.NewServer(normalizeTCPAddress(resolved.ABCIAddress), "socket", abciApp)
	if err != nil {
		_ = app.Close()
		return ReadyState{}, err
	}
	if err := abciSvc.Start(); err != nil {
		_ = app.Close()
		return ReadyState{}, err
	}
	cometNode, err := startCometNode(nodeHome, resolved, app)
	if err != nil {
		_ = abciSvc.Stop()
		_ = app.Close()
		return ReadyState{}, err
	}

	rpcLn, err := net.Listen("tcp", resolved.RPCAddress)
	if err != nil {
		_ = cometNode.Stop()
		_ = abciSvc.Stop()
		_ = app.Close()
		return ReadyState{}, err
	}
	grpcLn, err := net.Listen("tcp", resolved.GRPCAddress)
	if err != nil {
		_ = cometNode.Stop()
		_ = abciSvc.Stop()
		_ = rpcLn.Close()
		_ = app.Close()
		return ReadyState{}, err
	}

	state := ReadyState{
		Status:                 "ready",
		ChainID:                appCfg.ChainID,
		NodeID:                 nodeHome.NodeID,
		HomeDir:                appCfg.HomeDir,
		RPCAddress:             rpcLn.Addr().String(),
		CometRPCAddress:        resolved.CometRPCAddress,
		GRPCAddress:            grpcLn.Addr().String(),
		ABCIAddress:            normalizeTCPAddress(resolved.ABCIAddress),
		ConfigPath:             nodeHome.ConfigPath,
		CometGenesisPath:       nodeHome.CometGenesisPath,
		SDKGenesisPath:         nodeHome.SDKGenesisPath,
		NodeKeyPath:            nodeHome.NodeKeyPath,
		PrivValidatorKeyPath:   nodeHome.PrivValidatorKeyPath,
		PrivValidatorStatePath: nodeHome.PrivValidatorStatePath,
		CoreStoreKeys:          coreStoreKeys,
		SDKGenesisModules:      sdkGenesisModules,
	}

	node := DemoNode{
		appConfig:  appCfg,
		app:        app,
		config:     resolved,
		abciServer: abciSvc,
		cometNode:  cometNode,
		rpcLn:      rpcLn,
		grpcLn:     grpcLn,
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
	if n.cometNode != nil && n.cometNode.IsRunning() {
		n.cometNode.Stop()
	}
	if n.abciServer != nil && n.abciServer.IsRunning() {
		errs = appendCloseErr(errs, n.abciServer.Stop())
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

func startCometNode(nodeHome CometNodeHome, cfg Config, app *aegisapp.App) (*cmtnode.Node, error) {
	if err := bootstrapCometState(nodeHome, app); err != nil {
		return nil, err
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(nodeHome.Config.NodeKeyFile())
	if err != nil {
		return nil, err
	}

	cometNode, err := cmtnode.NewNode(
		nodeHome.Config,
		privval.LoadOrGenFilePV(nodeHome.Config.PrivValidatorKeyFile(), nodeHome.Config.PrivValidatorStateFile()),
		nodeKey,
		proxy.NewRemoteClientCreator(normalizeTCPAddress(cfg.ABCIAddress), "socket", true),
		cmtnode.DefaultGenesisDocProviderFunc(nodeHome.Config),
		cmtcfg.DefaultDBProvider,
		cmtnode.DefaultMetricsProvider(nodeHome.Config.Instrumentation),
		cmtlog.NewNopLogger(),
	)
	if err != nil {
		return nil, err
	}
	if err := cometNode.Start(); err != nil {
		return nil, err
	}
	if err := waitForCometRPC(cfg.CometRPCAddress, 5*time.Second); err != nil {
		cometNode.Stop()
		return nil, err
	}
	return cometNode, nil
}

func bootstrapCometState(nodeHome CometNodeHome, app *aegisapp.App) error {
	if app == nil {
		return nil
	}

	status := app.Status()
	if status.CurrentHeight == 0 {
		return nil
	}
	blockStoreDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "blockstore", Config: nodeHome.Config})
	if err != nil {
		return err
	}
	blockStore := cmtstore.NewBlockStore(blockStoreDB)
	defer blockStore.Close()
	if !blockStore.IsEmpty() {
		return nil
	}

	stateDB, err := cmtcfg.DefaultDBProvider(&cmtcfg.DBContext{ID: "state", Config: nodeHome.Config})
	if err != nil {
		return err
	}
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: nodeHome.Config.Storage.DiscardABCIResponses,
	})
	defer stateStore.Close()

	state, err := stateStore.Load()
	if err != nil {
		return err
	}
	if !state.IsEmpty() {
		return nil
	}

	appHash, err := hashABCIStatus(status)
	if err != nil {
		return err
	}
	genesisState, err := sm.MakeGenesisStateFromFile(nodeHome.Config.GenesisFile())
	if err != nil {
		return err
	}
	genesisState.LastBlockHeight = int64(status.CurrentHeight)
	genesisState.LastBlockTime = time.Now().UTC()
	genesisState.LastValidators = genesisState.Validators.Copy()
	genesisState.AppHash = appHash
	lastBlockID := cmttypes.BlockID{
		Hash: appHash,
		PartSetHeader: cmttypes.PartSetHeader{
			Total: 1,
			Hash:  appHash,
		},
	}
	genesisState.LastBlockID = lastBlockID
	if err := stateStore.Bootstrap(genesisState); err != nil {
		return err
	}
	if genesisState.LastBlockHeight > 0 && blockStore.LoadSeenCommit(genesisState.LastBlockHeight) == nil {
		filePV := privval.LoadFilePV(nodeHome.Config.PrivValidatorKeyFile(), nodeHome.Config.PrivValidatorStateFile())
		signatures := make([]cmttypes.CommitSig, len(genesisState.LastValidators.Validators))
		for idx := range signatures {
			signatures[idx] = cmttypes.NewCommitSigAbsent()
		}
		validatorIdx, _ := genesisState.LastValidators.GetByAddress(filePV.GetAddress())
		if validatorIdx >= 0 {
			vote := &cmtproto.Vote{
				Type:             cmtproto.PrecommitType,
				Height:           genesisState.LastBlockHeight,
				Round:            0,
				BlockID:          lastBlockID.ToProto(),
				Timestamp:        genesisState.LastBlockTime,
				ValidatorAddress: filePV.GetAddress(),
				ValidatorIndex:   int32(validatorIdx),
			}
			if err := filePV.SignVote(genesisState.ChainID, vote); err != nil {
				return err
			}
			signedVote, err := cmttypes.VoteFromProto(vote)
			if err != nil {
				return err
			}
			signatures[validatorIdx] = signedVote.CommitSig()
		}
		commit := &cmttypes.Commit{
			Height: genesisState.LastBlockHeight,
			Round:  0,
			BlockID: lastBlockID,
			Signatures: signatures,
		}
		if err := blockStore.SaveSeenCommit(genesisState.LastBlockHeight, commit); err != nil {
			return err
		}
	}
	return stateStore.SetOfflineStateSyncHeight(genesisState.LastBlockHeight)
}

func waitForCometRPC(address string, timeout time.Duration) error {
	client, err := rpchttp.New("http://"+strings.TrimSpace(address), "/websocket")
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err := client.Status(ctx)
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for comet rpc at %s", address)
}

func appendCloseErr(errs []error, err error) []error {
	if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EINVAL) {
		return errs
	}
	return append(errs, err)
}

func sortedGenesisModuleNames(genesisState map[string]json.RawMessage) []string {
	names := make([]string, 0, len(genesisState))
	for moduleName := range genesisState {
		names = append(names, moduleName)
	}
	sort.Strings(names)
	return names
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

func resolveConcreteTCPAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", nil
	}

	bare := strings.TrimPrefix(address, "tcp://")
	host, port, err := net.SplitHostPort(bare)
	if err != nil {
		return "", err
	}
	if port != "0" {
		return bare, nil
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return "", err
	}
	defer ln.Close()

	actualHost, actualPort, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return "", err
	}
	if _, err := strconv.Atoi(actualPort); err != nil {
		return "", err
	}
	return net.JoinHostPort(actualHost, actualPort), nil
}
