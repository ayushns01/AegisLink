package keeper

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ayushns01/aegislink/chain/aegislink/internal/sdkstore"
	ibcrouterkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/keeper"
	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	storetypes "cosmossdk.io/store/types"
)

var ErrInvalidProposal = errors.New("invalid governance proposal")

type ProposalKind string

const (
	ProposalKindAssetStatus ProposalKind = "asset_status"
	ProposalKindLimitUpdate ProposalKind = "limit_update"
	ProposalKindRoutePolicy ProposalKind = "route_policy"
)

type AssetStatusProposal struct {
	ProposalID string `json:"proposal_id"`
	AssetID    string `json:"asset_id"`
	Enabled    bool   `json:"enabled"`
}

func (p AssetStatusProposal) ValidateBasic() error {
	if strings.TrimSpace(p.ProposalID) == "" {
		return fmt.Errorf("%w: missing proposal id", ErrInvalidProposal)
	}
	if strings.TrimSpace(p.AssetID) == "" {
		return fmt.Errorf("%w: missing asset id", ErrInvalidProposal)
	}
	return nil
}

type LimitUpdateProposal struct {
	ProposalID string              `json:"proposal_id"`
	Limit      limittypes.RateLimit `json:"limit"`
}

func (p LimitUpdateProposal) ValidateBasic() error {
	if strings.TrimSpace(p.ProposalID) == "" {
		return fmt.Errorf("%w: missing proposal id", ErrInvalidProposal)
	}
	if err := p.Limit.ValidateBasic(); err != nil {
		return err
	}
	return nil
}

type RoutePolicyUpdateProposal struct {
	ProposalID string                    `json:"proposal_id"`
	RouteID    string                    `json:"route_id"`
	Policy     ibcroutertypes.RoutePolicy `json:"policy"`
}

func (p RoutePolicyUpdateProposal) ValidateBasic() error {
	if strings.TrimSpace(p.ProposalID) == "" {
		return fmt.Errorf("%w: missing proposal id", ErrInvalidProposal)
	}
	if strings.TrimSpace(p.RouteID) == "" {
		return fmt.Errorf("%w: missing route id", ErrInvalidProposal)
	}
	return nil
}

type ProposalRecord struct {
	ProposalID string       `json:"proposal_id"`
	Kind       ProposalKind `json:"kind"`
	TargetID   string       `json:"target_id"`
	Summary    string       `json:"summary"`
}

type StateSnapshot struct {
	AppliedProposals []ProposalRecord `json:"applied_proposals"`
}

type Keeper struct {
	registryKeeper  *registrykeeper.Keeper
	limitsKeeper    *limitskeeper.Keeper
	ibcRouterKeeper *ibcrouterkeeper.Keeper
	applied         []ProposalRecord
	stateStore      *sdkstore.JSONStateStore
}

func NewKeeper(
	registryKeeper *registrykeeper.Keeper,
	limitsKeeper *limitskeeper.Keeper,
	ibcRouterKeeper *ibcrouterkeeper.Keeper,
) *Keeper {
	return &Keeper{
		registryKeeper:  registryKeeper,
		limitsKeeper:    limitsKeeper,
		ibcRouterKeeper: ibcRouterKeeper,
		applied:         make([]ProposalRecord, 0),
	}
}

func NewStoreKeeper(
	multiStore storetypes.CommitMultiStore,
	key *storetypes.KVStoreKey,
	registryKeeper *registrykeeper.Keeper,
	limitsKeeper *limitskeeper.Keeper,
	ibcRouterKeeper *ibcrouterkeeper.Keeper,
) (*Keeper, error) {
	stateStore, err := sdkstore.NewJSONStateStore(multiStore, key)
	if err != nil {
		return nil, err
	}

	keeper := NewKeeper(registryKeeper, limitsKeeper, ibcRouterKeeper)
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

func (k *Keeper) ApplyAssetStatusProposal(proposal AssetStatusProposal) error {
	if err := proposal.ValidateBasic(); err != nil {
		return err
	}

	if proposal.Enabled {
		if err := k.registryKeeper.EnableAsset(proposal.AssetID); err != nil {
			return err
		}
	} else {
		if err := k.registryKeeper.DisableAsset(proposal.AssetID); err != nil {
			return err
		}
	}

	k.applied = append(k.applied, ProposalRecord{
		ProposalID: strings.TrimSpace(proposal.ProposalID),
		Kind:       ProposalKindAssetStatus,
		TargetID:   strings.TrimSpace(proposal.AssetID),
		Summary:    fmt.Sprintf("set asset %s enabled=%t", strings.TrimSpace(proposal.AssetID), proposal.Enabled),
	})
	return k.persist()
}

func (k *Keeper) ApplyLimitUpdateProposal(proposal LimitUpdateProposal) error {
	if err := proposal.ValidateBasic(); err != nil {
		return err
	}

	limit := proposal.Limit
	limit.MaxAmount = cloneAmount(limit.MaxAmount)
	if err := k.limitsKeeper.SetLimit(limit); err != nil {
		return err
	}

	k.applied = append(k.applied, ProposalRecord{
		ProposalID: strings.TrimSpace(proposal.ProposalID),
		Kind:       ProposalKindLimitUpdate,
		TargetID:   strings.TrimSpace(limit.AssetID),
		Summary:    fmt.Sprintf("set limit for %s to %s", strings.TrimSpace(limit.AssetID), limit.MaxAmount.String()),
	})
	return k.persist()
}

func (k *Keeper) ApplyRoutePolicyUpdateProposal(proposal RoutePolicyUpdateProposal) error {
	if err := proposal.ValidateBasic(); err != nil {
		return err
	}

	profile, ok := k.ibcRouterKeeper.GetRouteProfile(proposal.RouteID)
	if !ok {
		return ibcrouterkeeper.ErrRouteProfileNotFound
	}

	profile.Policy = proposal.Policy.Canonical()
	if err := k.ibcRouterKeeper.SetRouteProfile(profile); err != nil {
		return err
	}

	k.applied = append(k.applied, ProposalRecord{
		ProposalID: strings.TrimSpace(proposal.ProposalID),
		Kind:       ProposalKindRoutePolicy,
		TargetID:   strings.TrimSpace(proposal.RouteID),
		Summary:    fmt.Sprintf("updated route policy for %s", strings.TrimSpace(proposal.RouteID)),
	})
	return k.persist()
}

func (k *Keeper) ExportState() StateSnapshot {
	proposals := make([]ProposalRecord, 0, len(k.applied))
	for _, proposal := range k.applied {
		proposals = append(proposals, proposal)
	}
	return StateSnapshot{AppliedProposals: proposals}
}

func (k *Keeper) ImportState(state StateSnapshot) error {
	k.applied = make([]ProposalRecord, 0, len(state.AppliedProposals))
	for _, proposal := range state.AppliedProposals {
		k.applied = append(k.applied, ProposalRecord{
			ProposalID: strings.TrimSpace(proposal.ProposalID),
			Kind:       proposal.Kind,
			TargetID:   strings.TrimSpace(proposal.TargetID),
			Summary:    strings.TrimSpace(proposal.Summary),
		})
	}
	return k.persist()
}

func (k *Keeper) Flush() error {
	return k.persist()
}

func (k *Keeper) persist() error {
	if k.stateStore == nil {
		return nil
	}
	return k.stateStore.Save(k.ExportState())
}

func cloneAmount(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}
