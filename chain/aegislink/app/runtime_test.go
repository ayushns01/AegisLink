package app

import (
	"errors"
	"math/big"
	"path/filepath"
	"testing"

	bridgekeeper "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/keeper"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
)

func TestSaveAndLoadPreservesBridgeRuntimeState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "aegislink-state.json")
	app := NewWithConfig(Config{
		AppName:           AppName,
		Modules:           []string{"bridge", "registry", "limits", "pauser"},
		StatePath:         statePath,
		AllowedSigners:    []string{"relayer-1", "relayer-2", "relayer-3"},
		RequiredThreshold: 2,
	})

	asset := registrytypes.Asset{
		AssetID:        "eth.usdc",
		SourceChainID:  "11155111",
		SourceContract: "0xasset",
		Denom:          "uethusdc",
		Decimals:       6,
		DisplayName:    "USDC",
		Enabled:        true,
	}
	if err := app.RegisterAsset(asset); err != nil {
		t.Fatalf("register asset: %v", err)
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       asset.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustAmount(t, "1000000000000000000"),
	}); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	if err := app.Pause("maintenance"); err != nil {
		t.Fatalf("pause maintenance flow: %v", err)
	}

	claim := validDepositClaim(t)
	attestation := validAttestationForClaim(claim)
	app.SetCurrentHeight(50)
	if _, err := app.SubmitDepositClaim(claim, attestation); err != nil {
		t.Fatalf("submit deposit: %v", err)
	}

	app.SetCurrentHeight(60)
	withdrawal, err := app.ExecuteWithdrawal(claim.AssetID, claim.Amount, "0xrecipient", 120, []byte("threshold-proof"))
	if err != nil {
		t.Fatalf("execute withdrawal: %v", err)
	}

	if err := app.Save(); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if _, ok := loaded.RegistryKeeper.GetAsset(asset.AssetID); !ok {
		t.Fatalf("expected registered asset to persist")
	}
	if _, ok := loaded.LimitsKeeper.GetLimit(asset.AssetID); !ok {
		t.Fatalf("expected rate limit to persist")
	}
	if !loaded.PauserKeeper.IsPaused("maintenance") {
		t.Fatalf("expected paused maintenance flow to persist")
	}
	if supply := loaded.BridgeKeeper.SupplyForDenom(asset.Denom); supply.Sign() != 0 {
		t.Fatalf("expected burned supply to persist as zero, got %s", supply.String())
	}

	withdrawals := loaded.Withdrawals(60, 60)
	if len(withdrawals) != 1 {
		t.Fatalf("expected one persisted withdrawal, got %d", len(withdrawals))
	}
	if withdrawals[0].Identity.MessageID != withdrawal.Identity.MessageID {
		t.Fatalf("expected persisted withdrawal %q, got %q", withdrawal.Identity.MessageID, withdrawals[0].Identity.MessageID)
	}

	loaded.SetCurrentHeight(60)
	if _, err := loaded.SubmitDepositClaim(claim, attestation); !errors.Is(err, bridgekeeper.ErrDuplicateClaim) {
		t.Fatalf("expected duplicate claim rejection after reload, got %v", err)
	}
}

func validDepositClaim(t *testing.T) bridgetypes.DepositClaim {
	identity := bridgetypes.ClaimIdentity{
		Kind:           bridgetypes.ClaimKindDeposit,
		SourceChainID:  "11155111",
		SourceContract: "0xgateway",
		SourceTxHash:   "0xdeposit-tx",
		SourceLogIndex: 7,
		Nonce:          1,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount(t, "100000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           100,
	}
}

func validAttestationForClaim(claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	return bridgetypes.Attestation{
		MessageID:   claim.Identity.MessageID,
		PayloadHash: claim.Digest(),
		Signers:     []string{"relayer-1", "relayer-2"},
		Threshold:   2,
		Expiry:      120,
	}
}

func mustAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}
