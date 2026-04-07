package keeper

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	storetypes "cosmossdk.io/store/types"
)

var (
	ErrRouteNotFound          = errors.New("ibc route not found")
	ErrRouteDisabled          = errors.New("ibc route disabled")
	ErrInvalidRoute           = errors.New("invalid ibc route")
	ErrInvalidTransfer        = errors.New("invalid ibc transfer")
	ErrTransferNotFound       = errors.New("ibc transfer not found")
	ErrTransferNotPending     = errors.New("ibc transfer not pending")
	ErrTransferNotRecoverable = errors.New("ibc transfer not recoverable")
	ErrRouteProfileNotFound   = errors.New("ibc route profile not found")
	ErrRouteProfileDisabled   = errors.New("ibc route profile disabled")
	ErrRouteProfileAssetNotAllowed = errors.New("ibc route profile asset not allowed")
	ErrRouteProfilePolicyViolation = errors.New("ibc route profile policy violation")
)

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusAckFailed TransferStatus = "ack_failed"
	TransferStatusTimedOut  TransferStatus = "timed_out"
	TransferStatusRefunded  TransferStatus = "refunded"
)

type Route struct {
	AssetID            string `json:"asset_id"`
	DestinationChainID string `json:"destination_chain_id"`
	ChannelID          string `json:"channel_id"`
	DestinationDenom   string `json:"destination_denom"`
	Enabled            bool   `json:"enabled"`
}

func (r Route) ValidateBasic() error {
	if strings.TrimSpace(r.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidRoute)
	}
	if strings.TrimSpace(r.DestinationChainID) == "" {
		return fmt.Errorf("%w: missing destination chain id", ErrInvalidRoute)
	}
	if strings.TrimSpace(r.ChannelID) == "" {
		return fmt.Errorf("%w: missing channel id", ErrInvalidRoute)
	}
	if strings.TrimSpace(r.DestinationDenom) == "" {
		return fmt.Errorf("%w: missing destination denom", ErrInvalidRoute)
	}
	return nil
}

type TransferRecord struct {
	TransferID         string         `json:"transfer_id"`
	AssetID            string         `json:"asset_id"`
	Amount             *big.Int       `json:"amount"`
	Receiver           string         `json:"receiver"`
	DestinationChainID string         `json:"destination_chain_id"`
	ChannelID          string         `json:"channel_id"`
	DestinationDenom   string         `json:"destination_denom"`
	TimeoutHeight      uint64         `json:"timeout_height"`
	Memo               string         `json:"memo"`
	Status             TransferStatus `json:"status"`
	FailureReason      string         `json:"failure_reason"`
}

type StateSnapshot struct {
	NextSequence uint64                   `json:"next_sequence"`
	Routes       []Route                  `json:"routes"`
	RouteProfiles []ibcroutertypes.RouteProfile `json:"route_profiles"`
	Transfers    []TransferRecordSnapshot `json:"transfers"`
}

type TransferRecordSnapshot struct {
	TransferID         string         `json:"transfer_id"`
	AssetID            string         `json:"asset_id"`
	Amount             string         `json:"amount"`
	Receiver           string         `json:"receiver"`
	DestinationChainID string         `json:"destination_chain_id"`
	ChannelID          string         `json:"channel_id"`
	DestinationDenom   string         `json:"destination_denom"`
	TimeoutHeight      uint64         `json:"timeout_height"`
	Memo               string         `json:"memo"`
	Status             TransferStatus `json:"status"`
	FailureReason      string         `json:"failure_reason"`
}

type Keeper struct {
	routes       map[string]Route
	routeProfiles map[string]ibcroutertypes.RouteProfile
	transfers    map[string]TransferRecord
	nextSequence uint64
	stateStore   *sdkstore.JSONStateStore
}

func NewKeeper() *Keeper {
	return &Keeper{
		routes:       make(map[string]Route),
		routeProfiles: make(map[string]ibcroutertypes.RouteProfile),
		transfers:    make(map[string]TransferRecord),
		nextSequence: 1,
	}
}

func NewStoreKeeper(multiStore storetypes.CommitMultiStore, key *storetypes.KVStoreKey) (*Keeper, error) {
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper()
	keeper.stateStore = stateStore

	var state StateSnapshot
	if err := stateStore.Load(&state); err != nil {
		return nil, err
	}
	if err := keeper.ImportState(state); err != nil {
		return nil, err
	}

	return keeper, nil
}

func (k *Keeper) SetRoute(route Route) error {
	if err := route.ValidateBasic(); err != nil {
		return err
	}
	stored := canonicalRoute(route)
	k.routes[routeKey(stored.AssetID)] = stored
	return k.persist()
}

func (k *Keeper) SetRouteProfile(profile ibcroutertypes.RouteProfile) error {
	if err := profile.ValidateBasic(); err != nil {
		return err
	}
	stored := profile.Canonical()
	k.routeProfiles[routeProfileKey(stored.RouteID)] = stored
	return k.persist()
}

func (k *Keeper) GetRoute(assetID string) (Route, bool) {
	route, ok := k.routes[routeKey(assetID)]
	return route, ok
}

func (k *Keeper) GetRouteProfile(routeID string) (ibcroutertypes.RouteProfile, bool) {
	profile, ok := k.routeProfiles[routeProfileKey(routeID)]
	return profile, ok
}

func (k *Keeper) InitiateTransfer(assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo string) (TransferRecord, error) {
	route, ok := k.GetRoute(assetID)
	if !ok {
		return TransferRecord{}, ErrRouteNotFound
	}
	if !route.Enabled {
		return TransferRecord{}, ErrRouteDisabled
	}
	return k.initiateTransfer(assetID, amount, receiver, timeoutHeight, memo, route.DestinationChainID, route.ChannelID, route.DestinationDenom)
}

func (k *Keeper) InitiateTransferWithProfile(routeID, assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo string) (TransferRecord, error) {
	profile, ok := k.GetRouteProfile(routeID)
	if !ok {
		return TransferRecord{}, ErrRouteProfileNotFound
	}
	if !profile.Enabled {
		return TransferRecord{}, ErrRouteProfileDisabled
	}

	assetRoute, ok := profile.AssetRoute(assetID)
	if !ok {
		return TransferRecord{}, ErrRouteProfileAssetNotAllowed
	}
	if !profile.Policy.AllowsAction(memo) {
		return TransferRecord{}, ErrRouteProfilePolicyViolation
	}
	if !profile.Policy.AllowsMemo(memo) {
		return TransferRecord{}, ErrRouteProfilePolicyViolation
	}
	return k.initiateTransfer(assetID, amount, receiver, timeoutHeight, memo, profile.DestinationChainID, profile.ChannelID, assetRoute.DestinationDenom)
}

func (k *Keeper) initiateTransfer(assetID string, amount *big.Int, receiver string, timeoutHeight uint64, memo, destinationChainID, channelID, destinationDenom string) (TransferRecord, error) {
	if amount == nil || amount.Sign() <= 0 {
		return TransferRecord{}, fmt.Errorf("%w: amount must be positive", ErrInvalidTransfer)
	}
	if strings.TrimSpace(receiver) == "" {
		return TransferRecord{}, fmt.Errorf("%w: missing receiver", ErrInvalidTransfer)
	}
	if timeoutHeight == 0 {
		return TransferRecord{}, fmt.Errorf("%w: missing timeout height", ErrInvalidTransfer)
	}

	sequence := k.nextSequence
	k.nextSequence++
	record := TransferRecord{
		TransferID:         fmt.Sprintf("ibc/%s/%d", strings.TrimSpace(assetID), sequence),
		AssetID:            strings.TrimSpace(assetID),
		Amount:             cloneAmount(amount),
		Receiver:           strings.TrimSpace(receiver),
		DestinationChainID: strings.TrimSpace(destinationChainID),
		ChannelID:          strings.TrimSpace(channelID),
		DestinationDenom:   strings.TrimSpace(destinationDenom),
		TimeoutHeight:      timeoutHeight,
		Memo:               strings.TrimSpace(memo),
		Status:             TransferStatusPending,
	}
	k.transfers[record.TransferID] = record
	return cloneTransferRecord(record), k.persist()
}

func (k *Keeper) AcknowledgeSuccess(transferID string) (TransferRecord, error) {
	record, err := k.pendingTransfer(transferID)
	if err != nil {
		return TransferRecord{}, err
	}
	record.Status = TransferStatusCompleted
	record.FailureReason = ""
	k.transfers[record.TransferID] = record
	return cloneTransferRecord(record), k.persist()
}

func (k *Keeper) AcknowledgeFailure(transferID, reason string) (TransferRecord, error) {
	record, err := k.pendingTransfer(transferID)
	if err != nil {
		return TransferRecord{}, err
	}
	record.Status = TransferStatusAckFailed
	record.FailureReason = strings.TrimSpace(reason)
	k.transfers[record.TransferID] = record
	return cloneTransferRecord(record), k.persist()
}

func (k *Keeper) TimeoutTransfer(transferID string) (TransferRecord, error) {
	record, err := k.pendingTransfer(transferID)
	if err != nil {
		return TransferRecord{}, err
	}
	record.Status = TransferStatusTimedOut
	k.transfers[record.TransferID] = record
	return cloneTransferRecord(record), k.persist()
}

func (k *Keeper) MarkRefunded(transferID string) (TransferRecord, error) {
	record, ok := k.transfers[strings.TrimSpace(transferID)]
	if !ok {
		return TransferRecord{}, ErrTransferNotFound
	}
	if record.Status != TransferStatusAckFailed && record.Status != TransferStatusTimedOut {
		return TransferRecord{}, ErrTransferNotRecoverable
	}
	record.Status = TransferStatusRefunded
	k.transfers[record.TransferID] = record
	return cloneTransferRecord(record), k.persist()
}

func (k *Keeper) ExportRoutes() []Route {
	routes := make([]Route, 0, len(k.routes))
	for _, route := range k.routes {
		routes = append(routes, canonicalRoute(route))
	}
	return routes
}

func (k *Keeper) ExportRouteProfiles() []ibcroutertypes.RouteProfile {
	profiles := make([]ibcroutertypes.RouteProfile, 0, len(k.routeProfiles))
	for _, profile := range k.routeProfiles {
		profiles = append(profiles, profile.Canonical())
	}
	return profiles
}

func (k *Keeper) ExportTransfers() []TransferRecord {
	transfers := make([]TransferRecord, 0, len(k.transfers))
	for _, transfer := range k.transfers {
		transfers = append(transfers, cloneTransferRecord(transfer))
	}
	return transfers
}

func (k *Keeper) ExportState() StateSnapshot {
	state := StateSnapshot{
		NextSequence: k.nextSequence,
		Routes:       k.ExportRoutes(),
		RouteProfiles: k.ExportRouteProfiles(),
		Transfers:    make([]TransferRecordSnapshot, 0, len(k.transfers)),
	}
	for _, transfer := range k.transfers {
		state.Transfers = append(state.Transfers, TransferRecordSnapshot{
			TransferID:         transfer.TransferID,
			AssetID:            transfer.AssetID,
			Amount:             transfer.Amount.String(),
			Receiver:           transfer.Receiver,
			DestinationChainID: transfer.DestinationChainID,
			ChannelID:          transfer.ChannelID,
			DestinationDenom:   transfer.DestinationDenom,
			TimeoutHeight:      transfer.TimeoutHeight,
			Memo:               transfer.Memo,
			Status:             transfer.Status,
			FailureReason:      transfer.FailureReason,
		})
	}
	return state
}

func (k *Keeper) ImportState(state StateSnapshot) error {
	k.routes = make(map[string]Route, len(state.Routes))
	for _, route := range state.Routes {
		if err := k.SetRoute(route); err != nil {
			return err
		}
	}

	k.routeProfiles = make(map[string]ibcroutertypes.RouteProfile, len(state.RouteProfiles))
	for _, profile := range state.RouteProfiles {
		if err := k.SetRouteProfile(profile); err != nil {
			return err
		}
	}

	k.transfers = make(map[string]TransferRecord, len(state.Transfers))
	for _, transfer := range state.Transfers {
		amount, ok := new(big.Int).SetString(transfer.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid ibc transfer amount %q", transfer.Amount)
		}
		k.transfers[strings.TrimSpace(transfer.TransferID)] = TransferRecord{
			TransferID:         strings.TrimSpace(transfer.TransferID),
			AssetID:            strings.TrimSpace(transfer.AssetID),
			Amount:             amount,
			Receiver:           strings.TrimSpace(transfer.Receiver),
			DestinationChainID: strings.TrimSpace(transfer.DestinationChainID),
			ChannelID:          strings.TrimSpace(transfer.ChannelID),
			DestinationDenom:   strings.TrimSpace(transfer.DestinationDenom),
			TimeoutHeight:      transfer.TimeoutHeight,
			Memo:               strings.TrimSpace(transfer.Memo),
			Status:             transfer.Status,
			FailureReason:      strings.TrimSpace(transfer.FailureReason),
		}
	}

	k.nextSequence = state.NextSequence
	if k.nextSequence == 0 {
		k.nextSequence = 1
	}
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.stateStore == nil {
		return nil
	}
	return k.stateStore.Save(k.ExportState())
}

func (k *Keeper) Flush() error {
	return k.persist()
}

func (k *Keeper) pendingTransfer(transferID string) (TransferRecord, error) {
	record, ok := k.transfers[strings.TrimSpace(transferID)]
	if !ok {
		return TransferRecord{}, ErrTransferNotFound
	}
	if record.Status != TransferStatusPending {
		return TransferRecord{}, ErrTransferNotPending
	}
	return record, nil
}

func routeKey(assetID string) string {
	return strings.TrimSpace(assetID)
}

func routeProfileKey(routeID string) string {
	return strings.TrimSpace(routeID)
}

func canonicalRoute(route Route) Route {
	route.AssetID = strings.TrimSpace(route.AssetID)
	route.DestinationChainID = strings.TrimSpace(route.DestinationChainID)
	route.ChannelID = strings.TrimSpace(route.ChannelID)
	route.DestinationDenom = strings.TrimSpace(route.DestinationDenom)
	return route
}

func cloneAmount(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(value)
}

func cloneTransferRecord(record TransferRecord) TransferRecord {
	record.TransferID = strings.TrimSpace(record.TransferID)
	record.AssetID = strings.TrimSpace(record.AssetID)
	record.Amount = cloneAmount(record.Amount)
	record.Receiver = strings.TrimSpace(record.Receiver)
	record.DestinationChainID = strings.TrimSpace(record.DestinationChainID)
	record.ChannelID = strings.TrimSpace(record.ChannelID)
	record.DestinationDenom = strings.TrimSpace(record.DestinationDenom)
	record.Memo = strings.TrimSpace(record.Memo)
	record.FailureReason = strings.TrimSpace(record.FailureReason)
	return record
}
