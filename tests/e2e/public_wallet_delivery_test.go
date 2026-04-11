package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aegisapp "github.com/ayushns01/aegislink/chain/aegislink/app"
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
	limittypes "github.com/ayushns01/aegislink/chain/aegislink/x/limits/types"
	registrytypes "github.com/ayushns01/aegislink/chain/aegislink/x/registry/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPublicWalletDelivery(t *testing.T) {
	t.Parallel()

	homeDir := filepath.Join(t.TempDir(), "public-wallet-home")
	cfg, err := aegisapp.InitHome(aegisapp.Config{
		HomeDir:     homeDir,
		ChainID:     "aegislink-public-1",
		RuntimeMode: aegisapp.RuntimeModeSDKStore,
	}, false)
	if err != nil {
		t.Fatalf("init home: %v", err)
	}

	app, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}

	recipient := sdk.AccAddress([]byte("wallet-bridge-h1")).String()
	if err := seedPublicWalletAssets(t, app, recipient); err != nil {
		t.Fatalf("seed assets: %v", err)
	}
	if err := app.Save(); err != nil {
		t.Fatalf("save seeded state: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close seeded runtime: %v", err)
	}

	reloaded, err := aegisapp.LoadWithConfig(cfg)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	persistedBalances, err := reloaded.WalletBalances(recipient)
	if err != nil {
		t.Fatalf("load wallet balances: %v", err)
	}
	if len(persistedBalances) != 2 {
		t.Fatalf("expected two wallet balances after reload, got %d (%+v)", len(persistedBalances), persistedBalances)
	}
	gotByDenom := make(map[string]string, len(persistedBalances))
	for _, balance := range persistedBalances {
		gotByDenom[balance.Denom] = balance.Amount
	}
	if gotByDenom["ueth"] != "1000000000000000000" {
		t.Fatalf("expected bridged ETH balance to persist, got %+v", gotByDenom)
	}
	if gotByDenom["uethusdc"] != "25000000" {
		t.Fatalf("expected bridged ERC-20 balance to persist, got %+v", gotByDenom)
	}
	if err := reloaded.Close(); err != nil {
		t.Fatalf("close reloaded runtime: %v", err)
	}

	output := runGoCommandWithLocalCache(t, repoRoot(t), "run", "./chain/aegislink/cmd/aegislinkd", "query", "balances", "--home", homeDir, "--address", recipient)

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := decodeLastJSONObject(output, &balances); err != nil {
		t.Fatalf("decode balance query output: %v\n%s", err, output)
	}
	if len(balances) != 2 {
		t.Fatalf("expected two wallet balances, got %d (%+v)", len(balances), balances)
	}
}

func seedPublicWalletAssets(t *testing.T, app *aegisapp.App, recipient string) error {
	t.Helper()

	nativeETH := registrytypes.Asset{
		AssetID:         "eth",
		SourceChainID:   "11155111",
		SourceAssetKind: registrytypes.SourceAssetKindNativeETH,
		Denom:           "ueth",
		Decimals:        18,
		DisplayName:     "Ether",
		DisplaySymbol:   "ETH",
		Enabled:         true,
	}
	if err := app.RegisterAsset(nativeETH); err != nil {
		return err
	}

	erc20 := registrytypes.Asset{
		AssetID:            "eth.usdc",
		SourceChainID:      "11155111",
		SourceAssetKind:    registrytypes.SourceAssetKindERC20,
		SourceAssetAddress: "0xusdc",
		Denom:              "uethusdc",
		Decimals:           6,
		DisplayName:        "USD Coin",
		DisplaySymbol:      "USDC",
		Enabled:            true,
	}
	if err := app.RegisterAsset(erc20); err != nil {
		return err
	}

	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       nativeETH.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustWalletAmount(t, "2000000000000000000"),
	}); err != nil {
		return err
	}
	if err := app.SetLimit(limittypes.RateLimit{
		AssetID:       erc20.AssetID,
		WindowSeconds: 600,
		MaxAmount:     mustWalletAmount(t, "100000000"),
	}); err != nil {
		return err
	}

	nativeClaim := depositClaim(t, bridgetypes.SourceAssetKindNativeETH, "", "eth", "0xnative-deposit", 1, 1, recipient, "1000000000000000000")
	if err := submitClaim(t, app, nativeClaim); err != nil {
		return err
	}

	erc20Claim := depositClaim(t, bridgetypes.SourceAssetKindERC20, "0xusdc", "eth.usdc", "0xerc20-deposit", 2, 2, recipient, "25000000")
	if err := submitClaim(t, app, erc20Claim); err != nil {
		return err
	}

	return nil
}

func submitClaim(t *testing.T, app *aegisapp.App, claim bridgetypes.DepositClaim) error {
	t.Helper()

	attestation := testAttestationForClaim(t, claim)
	_, err := app.SubmitDepositClaim(claim, attestation)
	return err
}

func depositClaim(t *testing.T, sourceAssetKind, sourceContract, assetID, txHash string, logIndex, nonce uint64, recipient, amount string) bridgetypes.DepositClaim {
	t.Helper()

	identity := bridgetypes.ClaimIdentity{
		Kind:            bridgetypes.ClaimKindDeposit,
		SourceAssetKind: sourceAssetKind,
		SourceChainID:   "11155111",
		SourceContract:  sourceContract,
		SourceTxHash:    txHash,
		SourceLogIndex:  logIndex,
		Nonce:           nonce,
	}
	identity.MessageID = identity.DerivedMessageID()

	return bridgetypes.DepositClaim{
		Identity:           identity,
		DestinationChainID: "aegislink-public-1",
		AssetID:            assetID,
		Amount:             mustWalletAmount(t, amount),
		Recipient:          recipient,
		Deadline:           120,
	}
}

func testAttestationForClaim(t *testing.T, claim bridgetypes.DepositClaim) bridgetypes.Attestation {
	t.Helper()

	attestation := bridgetypes.Attestation{
		MessageID:        claim.Identity.MessageID,
		PayloadHash:      claim.Digest(),
		Signers:          bridgetypes.DefaultHarnessSignerAddresses()[:2],
		Threshold:        2,
		Expiry:           200,
		SignerSetVersion: 1,
	}
	for _, key := range bridgetypes.DefaultHarnessSignerPrivateKeys()[:2] {
		proof, err := bridgetypes.SignAttestationWithPrivateKeyHex(attestation, key)
		if err != nil {
			t.Fatalf("sign attestation: %v", err)
		}
		attestation.Proofs = append(attestation.Proofs, proof)
	}
	return attestation
}

func mustWalletAmount(t *testing.T, value string) *big.Int {
	t.Helper()

	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		t.Fatalf("invalid amount %q", value)
	}
	return amount
}

func decodeWalletBalances(t *testing.T, raw string) []struct {
	Address string `json:"address"`
	Denom   string `json:"denom"`
	Amount  string `json:"amount"`
} {
	t.Helper()

	var balances []struct {
		Address string `json:"address"`
		Denom   string `json:"denom"`
		Amount  string `json:"amount"`
	}
	if err := json.Unmarshal([]byte(raw), &balances); err != nil {
		t.Fatalf("decode balances fixture: %v", err)
	}
	return balances
}

func testCosmosWalletAddress() string {
	return sdk.AccAddress([]byte("e2e-wallet-recipient")).String()
}

func runGoCommandWithLocalCache(t *testing.T, dir string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, os.Environ()...)
	cmd.Env = append(cmd.Env,
		"GOCACHE=/tmp/aegislink-gocache",
		"GOMODCACHE=/Users/ayushns01/go/pkg/mod",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("command timed out: go %s\n%s", strings.Join(args, " "), output)
		}
		t.Fatalf("command failed: go %s\n%s", strings.Join(args, " "), output)
	}
	return string(output)
}
