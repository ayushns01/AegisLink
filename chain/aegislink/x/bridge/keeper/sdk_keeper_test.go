package keeper

import (
	"math/big"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/ayushns01/aegislink/chain/aegislink/testutil"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limitskeeper "github.com/ayushns01/aegislink/chain/aegislink/x/limits/keeper"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	pauserkeeper "github.com/ayushns01/aegislink/chain/aegislink/x/pauser/keeper"
	registrykeeper "github.com/ayushns01/aegislink/chain/aegislink/x/registry/keeper"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestSDKKeeperPersistsBridgeAccountingAcrossReload(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "bridge", "registry", "limits", "pauser")

	registry, err := registrykeeper.NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed registry keeper, got %v", err)
	}
	limits, err := limitskeeper.NewStoreKeeper(store, keys["limits"])
	if err != nil {
		t.Fatalf("expected store-backed limits keeper, got %v", err)
	}
	pauser, err := pauserkeeper.NewStoreKeeper(store, keys["pauser"])
	if err != nil {
		t.Fatalf("expected store-backed pauser keeper, got %v", err)
	}

	_, claim, attestation, _, _, _ := newKeeperFixture(t)
	storeKeeper, err := NewStoreKeeper(store, keys["bridge"], registry, limits, pauser, bridgetypes.DefaultHarnessSignerAddresses()[:3], 2)
	if err != nil {
		t.Fatalf("expected store-backed bridge keeper, got %v", err)
	}

	if err := registry.RegisterAsset(registrytypes.Asset{
		AssetID:        claim.AssetID,
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("expected asset registration to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       claim.AssetID,
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(250000000),
	}); err != nil {
		t.Fatalf("expected limit registration to succeed, got %v", err)
	}

	if _, err := storeKeeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected deposit claim to succeed, got %v", err)
	}

	reloaded, err := NewStoreKeeper(store, keys["bridge"], registry, limits, pauser, bridgetypes.DefaultHarnessSignerAddresses()[:3], 2)
	if err != nil {
		t.Fatalf("expected bridge keeper reload to succeed, got %v", err)
	}

	state := reloaded.ExportState()
	if len(state.ProcessedClaims) != 1 {
		t.Fatalf("expected one processed claim after reload, got %d", len(state.ProcessedClaims))
	}
	if supply := reloaded.SupplyForDenom("uethusdc"); supply.Cmp(claim.Amount) != 0 {
		t.Fatalf("expected supply %s after reload, got %s", claim.Amount.String(), supply.String())
	}
}

func TestSDKKeeperStoresClaimsAndSupplyUnderPrefixKeys(t *testing.T) {
	store, keys := testutil.NewInMemoryCommitMultiStore(t, "bridge", "registry", "limits", "pauser")

	registry, err := registrykeeper.NewStoreKeeper(store, keys["registry"])
	if err != nil {
		t.Fatalf("expected store-backed registry keeper, got %v", err)
	}
	limits, err := limitskeeper.NewStoreKeeper(store, keys["limits"])
	if err != nil {
		t.Fatalf("expected store-backed limits keeper, got %v", err)
	}
	pauser, err := pauserkeeper.NewStoreKeeper(store, keys["pauser"])
	if err != nil {
		t.Fatalf("expected store-backed pauser keeper, got %v", err)
	}

	keeper, claim, attestation, _, _, _ := newKeeperFixture(t)
	storeKeeper, err := NewStoreKeeper(store, keys["bridge"], registry, limits, pauser, bridgetypes.DefaultHarnessSignerAddresses()[:3], 2)
	if err != nil {
		t.Fatalf("expected store-backed bridge keeper, got %v", err)
	}
	if err := registry.RegisterAsset(registrytypes.Asset{
		AssetID:        claim.AssetID,
		SourceChainID:  "ethereum-sepolia",
		SourceContract: "0xabc123",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "Ethereum USDC",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("expected asset registration to succeed, got %v", err)
	}
	if err := limits.SetLimit(limittypes.RateLimit{
		AssetID:       claim.AssetID,
		WindowSeconds: 600,
		MaxAmount:     big.NewInt(250000000),
	}); err != nil {
		t.Fatalf("expected limit registration to succeed, got %v", err)
	}
	if _, err := storeKeeper.ExecuteDepositClaim(claim, attestation); err != nil {
		t.Fatalf("expected deposit claim to succeed, got %v", err)
	}

	kv := store.GetKVStore(keys["bridge"])
	if raw := kv.Get([]byte("state")); len(raw) != 0 {
		t.Fatalf("expected legacy state blob to be absent, got %q", string(raw))
	}
	claimIter := storetypes.KVStorePrefixIterator(kv, []byte("claim/"))
	defer claimIter.Close()
	if !claimIter.Valid() {
		t.Fatal("expected at least one claim/ prefix record")
	}
	supplyIter := storetypes.KVStorePrefixIterator(kv, []byte("supply/"))
	defer supplyIter.Close()
	if !supplyIter.Valid() {
		t.Fatal("expected at least one supply/ prefix record")
	}

	_ = keeper
}
