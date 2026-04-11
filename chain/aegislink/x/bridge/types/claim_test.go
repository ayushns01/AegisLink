package types

import (
	"errors"
	"math/big"
	"testing"
)

func TestReplayKeyIsDeterministicAndCanonical(t *testing.T) {
	base := ClaimIdentity{
		Kind:            ClaimKindDeposit,
		SourceAssetKind: SourceAssetKindERC20,
		SourceChainID:   "ethereum-1",
		SourceContract:  "0xabc|123",
		SourceTxHash:    "0xdeadbeef",
		SourceLogIndex:  17,
		Nonce:           42,
	}

	first := base.ReplayKey()
	second := base.ReplayKey()

	if first == "" {
		t.Fatal("expected replay key to be non-empty")
	}
	if first != second {
		t.Fatalf("expected deterministic replay key, got %q and %q", first, second)
	}

	whitespaceVariant := ClaimIdentity{
		Kind:            ClaimKindDeposit,
		SourceAssetKind: SourceAssetKindERC20,
		SourceChainID:   "  ethereum-1  ",
		SourceContract:  "\n0xabc|123\t",
		SourceTxHash:    " 0xdeadbeef ",
		SourceLogIndex:  17,
		Nonce:           42,
	}
	if first != whitespaceVariant.ReplayKey() {
		t.Fatalf("expected whitespace variants to normalize to the same replay key, got %q and %q", first, whitespaceVariant.ReplayKey())
	}

	delimiterCollision := ClaimIdentity{
		Kind:            ClaimKindDeposit,
		SourceAssetKind: SourceAssetKindERC20,
		SourceChainID:   "ethereum-1|extra",
		SourceContract:  "0xabc123",
		SourceTxHash:    "0xdeadbeef",
		SourceLogIndex:  17,
		Nonce:           42,
	}
	if first == delimiterCollision.ReplayKey() {
		t.Fatal("expected canonical encoding to avoid separator collisions")
	}

	base.SourceLogIndex = 18
	if first == base.ReplayKey() {
		t.Fatal("expected replay key to change when source identity changes")
	}
}

func TestClaimIdentityValidateBasic(t *testing.T) {
	identity := validClaimIdentity(ClaimKindDeposit)
	if err := identity.ValidateBasic(); err != nil {
		t.Fatalf("expected valid identity, got error: %v", err)
	}

	t.Run("legacy claims may omit source asset kind", func(t *testing.T) {
		identity := validLegacyClaimIdentity(ClaimKindDeposit)
		if err := identity.ValidateBasic(); err != nil {
			t.Fatalf("expected valid legacy identity, got error: %v", err)
		}
	})

	t.Run("native eth may omit source contract", func(t *testing.T) {
		identity := validNativeClaimIdentity(ClaimKindDeposit)
		if err := identity.ValidateBasic(); err != nil {
			t.Fatalf("expected valid native identity, got error: %v", err)
		}
	})

	t.Run("missing message id", func(t *testing.T) {
		identity := validClaimIdentity(ClaimKindDeposit)
		identity.MessageID = ""
		if err := identity.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
			t.Fatalf("expected invalid claim error, got: %v", err)
		}
	})

	t.Run("message id mismatch", func(t *testing.T) {
		identity := validClaimIdentity(ClaimKindDeposit)
		identity.MessageID = "wrong"
		if err := identity.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
			t.Fatalf("expected invalid claim error, got: %v", err)
		}
	})

	t.Run("missing source contract", func(t *testing.T) {
		identity := validClaimIdentity(ClaimKindDeposit)
		identity.SourceContract = ""
		identity.MessageID = identity.DerivedMessageID()
		if err := identity.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
			t.Fatalf("expected invalid claim error, got: %v", err)
		}
	})

	t.Run("erc20 requires source contract", func(t *testing.T) {
		identity := validClaimIdentity(ClaimKindDeposit)
		identity.SourceContract = ""
		identity.MessageID = identity.DerivedMessageID()
		if err := identity.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
			t.Fatalf("expected invalid claim error, got: %v", err)
		}
	})
}

func TestDepositClaimValidateBasic(t *testing.T) {
	claim := DepositClaim{
		Identity:           validClaimIdentity(ClaimKindDeposit),
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount("100000000000000000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           99,
	}

	if err := claim.ValidateBasic(); err != nil {
		t.Fatalf("expected valid deposit claim, got error: %v", err)
	}

	tests := map[string]func(*DepositClaim){
		"missing destination chain id": func(c *DepositClaim) { c.DestinationChainID = "" },
		"missing asset id":             func(c *DepositClaim) { c.AssetID = "" },
		"missing amount":               func(c *DepositClaim) { c.Amount = nil },
		"missing recipient":            func(c *DepositClaim) { c.Recipient = "" },
		"missing deadline":             func(c *DepositClaim) { c.Deadline = 0 },
		"missing message id": func(c *DepositClaim) {
			c.Identity.MessageID = ""
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			claim := claim
			mutate(&claim)
			if err := claim.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
				t.Fatalf("expected invalid claim error, got: %v", err)
			}
		})
	}
}

func TestDepositClaimDigestLocksTransferPayload(t *testing.T) {
	base := DepositClaim{
		Identity:           validClaimIdentity(ClaimKindDeposit),
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount("100000000000000000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           99,
	}

	first := base.Digest()
	if first == "" {
		t.Fatal("expected digest to be non-empty")
	}

	sameWithWhitespace := base
	sameWithWhitespace.DestinationChainID = "  aegislink-1 "
	sameWithWhitespace.AssetID = "\neth.usdc\t"
	sameWithWhitespace.Recipient = " cosmos1recipient "
	if first != sameWithWhitespace.Digest() {
		t.Fatal("expected digest normalization to be stable across whitespace-only variants")
	}

	changedAmount := base
	changedAmount.Amount = mustAmount("100000000000000000001")
	if first == changedAmount.Digest() {
		t.Fatal("expected digest to change when amount changes")
	}

	changedRecipient := base
	changedRecipient.Recipient = "cosmos1another"
	if first == changedRecipient.Digest() {
		t.Fatal("expected digest to change when recipient changes")
	}

	changedDeadline := base
	changedDeadline.Deadline = 100
	if first == changedDeadline.Digest() {
		t.Fatal("expected digest to change when deadline changes")
	}
}

func TestWithdrawalClaimValidateBasic(t *testing.T) {
	claim := WithdrawalClaim{
		Identity:           validClaimIdentity(ClaimKindWithdrawal),
		DestinationChainID: "ethereum-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount("340282366920938463463374607431768211456"),
		Recipient:          "0xrecipient",
		Deadline:           100,
	}

	if err := claim.ValidateBasic(); err != nil {
		t.Fatalf("expected valid withdrawal claim, got error: %v", err)
	}

	claim.Amount = nil
	if err := claim.ValidateBasic(); !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("expected invalid claim error for nil amount, got: %v", err)
	}
}

func TestAttestationValidateBasic(t *testing.T) {
	attestation := Attestation{
		MessageID:   validClaimIdentity(ClaimKindDeposit).MessageID,
		PayloadHash: validDepositClaim().Digest(),
		Signers:     []string{"signer-1", "signer-2", "signer-3"},
		Proofs: []AttestationProof{
			{Signer: "signer-1", Signature: []byte{1}},
			{Signer: "signer-2", Signature: []byte{2}},
			{Signer: "signer-3", Signature: []byte{3}},
		},
		Threshold:        2,
		Expiry:           123,
		SignerSetVersion: 1,
	}

	if err := attestation.ValidateBasic(); err != nil {
		t.Fatalf("expected valid attestation, got error: %v", err)
	}

	cases := map[string]func(*Attestation){
		"missing message id":         func(a *Attestation) { a.MessageID = "" },
		"missing payload hash":       func(a *Attestation) { a.PayloadHash = "" },
		"missing proofs":             func(a *Attestation) { a.Proofs = nil },
		"missing threshold":          func(a *Attestation) { a.Threshold = 0 },
		"missing expiry":             func(a *Attestation) { a.Expiry = 0 },
		"missing signer set version": func(a *Attestation) { a.SignerSetVersion = 0 },
		"threshold overflow":         func(a *Attestation) { a.Threshold = 4 },
		"duplicate proof signer": func(a *Attestation) {
			a.Proofs = []AttestationProof{
				{Signer: "signer-1", Signature: []byte{1}},
				{Signer: "signer-1", Signature: []byte{2}},
			}
		},
		"missing proof signature": func(a *Attestation) {
			a.Proofs = []AttestationProof{
				{Signer: "signer-1", Signature: nil},
				{Signer: "signer-2", Signature: []byte{2}},
			}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			attestation := attestation
			mutate(&attestation)
			if err := attestation.ValidateBasic(); !errors.Is(err, ErrInvalidAttestation) {
				t.Fatalf("expected invalid attestation error, got: %v", err)
			}
		})
	}
}

func validClaimIdentity(kind ClaimKind) ClaimIdentity {
	identity := ClaimIdentity{
		Kind:            kind,
		SourceAssetKind: SourceAssetKindERC20,
		SourceChainID:   "ethereum-1",
		SourceContract:  "0xabc123",
		SourceTxHash:    "0xdeadbeef",
		SourceLogIndex:  17,
		Nonce:           42,
	}
	identity.MessageID = identity.DerivedMessageID()
	return identity
}

func validLegacyClaimIdentity(kind ClaimKind) ClaimIdentity {
	identity := ClaimIdentity{
		Kind:           kind,
		SourceChainID:  "ethereum-1",
		SourceContract: "0xabc123",
		SourceTxHash:   "0xdeadbeef",
		SourceLogIndex: 17,
		Nonce:          42,
	}
	identity.MessageID = identity.DerivedMessageID()
	return identity
}

func validNativeClaimIdentity(kind ClaimKind) ClaimIdentity {
	identity := ClaimIdentity{
		Kind:            kind,
		SourceAssetKind: SourceAssetKindNativeETH,
		SourceChainID:   "ethereum-1",
		SourceTxHash:    "0xdeadbeef",
		SourceLogIndex:  17,
		Nonce:           42,
	}
	identity.MessageID = identity.DerivedMessageID()
	return identity
}

func validDepositClaim() DepositClaim {
	return DepositClaim{
		Identity:           validClaimIdentity(ClaimKindDeposit),
		DestinationChainID: "aegislink-1",
		AssetID:            "eth.usdc",
		Amount:             mustAmount("100000000000000000000"),
		Recipient:          "cosmos1recipient",
		Deadline:           99,
	}
}

func mustAmount(value string) *big.Int {
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid test amount")
	}
	return amount
}
