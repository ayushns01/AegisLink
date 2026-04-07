package keeper

import (
	"errors"
	"testing"

	ibcroutertypes "github.com/ayushns01/aegislink/chain/aegislink/x/ibcrouter/types"
)

func TestRouteProfileSupportsMultipleDestinationsForSameAsset(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:"},
		},
	}); err != nil {
		t.Fatalf("set osmosis route profile: %v", err)
	}
	if err := k.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "neutron-yield",
		DestinationChainID: "neutron-1",
		ChannelID:          "channel-7",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "factory/neutron/usdc"},
		},
	}); err != nil {
		t.Fatalf("set neutron route profile: %v", err)
	}

	osmoTransfer, err := k.InitiateTransferWithProfile("osmosis-fast", "eth.usdc", mustAmount("25000000"), "osmo1receiver", 120, "swap:uosmo")
	if err != nil {
		t.Fatalf("initiate osmosis transfer: %v", err)
	}
	if osmoTransfer.DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis destination chain, got %q", osmoTransfer.DestinationChainID)
	}
	if osmoTransfer.ChannelID != "channel-0" {
		t.Fatalf("expected osmosis channel, got %q", osmoTransfer.ChannelID)
	}
	if osmoTransfer.DestinationDenom != "ibc/uethusdc" {
		t.Fatalf("expected osmosis denom, got %q", osmoTransfer.DestinationDenom)
	}

	neutronTransfer, err := k.InitiateTransferWithProfile("neutron-yield", "eth.usdc", mustAmount("25000000"), "neutron1receiver", 120, "")
	if err != nil {
		t.Fatalf("initiate neutron transfer: %v", err)
	}
	if neutronTransfer.DestinationChainID != "neutron-1" {
		t.Fatalf("expected neutron destination chain, got %q", neutronTransfer.DestinationChainID)
	}
	if neutronTransfer.ChannelID != "channel-7" {
		t.Fatalf("expected neutron channel, got %q", neutronTransfer.ChannelID)
	}
	if neutronTransfer.DestinationDenom != "factory/neutron/usdc" {
		t.Fatalf("expected neutron denom, got %q", neutronTransfer.DestinationDenom)
	}
}

func TestRouteProfileRejectsAssetOutsideAllowlist(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
	}); err != nil {
		t.Fatalf("set route profile: %v", err)
	}

	_, err := k.InitiateTransferWithProfile("osmosis-fast", "eth.weth", mustAmount("1"), "osmo1receiver", 120, "")
	if !errors.Is(err, ErrRouteProfileAssetNotAllowed) {
		t.Fatalf("expected asset allowlist rejection, got %v", err)
	}
}

func TestRouteProfileEnforcesMemoPrefixPolicy(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-fast",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedMemoPrefixes: []string{"swap:"},
		},
	}); err != nil {
		t.Fatalf("set route profile: %v", err)
	}

	_, err := k.InitiateTransferWithProfile("osmosis-fast", "eth.usdc", mustAmount("1"), "osmo1receiver", 120, "stake:uosmo")
	if !errors.Is(err, ErrRouteProfilePolicyViolation) {
		t.Fatalf("expected policy violation, got %v", err)
	}

	record, err := k.InitiateTransferWithProfile("osmosis-fast", "eth.usdc", mustAmount("1"), "osmo1receiver", 120, "swap:uosmo")
	if err != nil {
		t.Fatalf("expected allowed memo to pass, got %v", err)
	}
	if record.DestinationChainID != "osmosis-1" {
		t.Fatalf("expected osmosis destination chain, got %q", record.DestinationChainID)
	}
}

func TestRouteProfileRestrictsAllowedActionTypes(t *testing.T) {
	t.Parallel()

	k := NewKeeper()
	if err := k.SetRouteProfile(ibcroutertypes.RouteProfile{
		RouteID:            "osmosis-stake",
		DestinationChainID: "osmosis-1",
		ChannelID:          "channel-0",
		Enabled:            true,
		Assets: []ibcroutertypes.AssetRoute{
			{AssetID: "eth.usdc", DestinationDenom: "ibc/uethusdc"},
		},
		Policy: ibcroutertypes.RoutePolicy{
			AllowedActionTypes: []string{"stake"},
		},
	}); err != nil {
		t.Fatalf("set route profile: %v", err)
	}

	_, err := k.InitiateTransferWithProfile("osmosis-stake", "eth.usdc", mustAmount("1"), "osmo1receiver", 120, "swap:uosmo")
	if !errors.Is(err, ErrRouteProfilePolicyViolation) {
		t.Fatalf("expected route action rejection, got %v", err)
	}

	record, err := k.InitiateTransferWithProfile("osmosis-stake", "eth.usdc", mustAmount("1"), "osmo1receiver", 120, "stake:ibc/uethusdc")
	if err != nil {
		t.Fatalf("expected stake action to pass, got %v", err)
	}
	if record.Memo != "stake:ibc/uethusdc" {
		t.Fatalf("expected stake memo to persist, got %q", record.Memo)
	}
}
