// Package testutil provides cryptographic helpers for use in tests only.
// This package MUST NOT be imported from production code paths.
// It contains well-known private keys (sequential integers 1–6) that are
// intentionally predictable for deterministic testing; their compromise
// has no security consequence because they are never used on live networks.
package testutil

import (
	bridgetypes "github.com/ayushns01/aegislink/chain/aegislink/x/bridge/types"
)

// HarnessSigner is a test-only signer with a known address and private key.
type HarnessSigner struct {
	Address       string
	PrivateKeyHex string
}

// defaultHarnessSignerPrivateKeys are the secp256k1 private keys for the
// integers 1–6. They are trivially known; use only in tests and local devnets.
var defaultHarnessSignerPrivateKeys = []string{
	"0000000000000000000000000000000000000000000000000000000000000001",
	"0000000000000000000000000000000000000000000000000000000000000002",
	"0000000000000000000000000000000000000000000000000000000000000003",
	"0000000000000000000000000000000000000000000000000000000000000004",
	"0000000000000000000000000000000000000000000000000000000000000005",
	"0000000000000000000000000000000000000000000000000000000000000006",
}

// DefaultHarnessAttestationSigners returns test signers with known addresses and keys.
func DefaultHarnessAttestationSigners() []HarnessSigner {
	signers := make([]HarnessSigner, 0, len(defaultHarnessSignerPrivateKeys))
	for _, key := range defaultHarnessSignerPrivateKeys {
		address, err := bridgetypes.SignerAddressFromPrivateKeyHex(key)
		if err != nil {
			panic(err)
		}
		signers = append(signers, HarnessSigner{
			Address:       address,
			PrivateKeyHex: key,
		})
	}
	return signers
}

// DefaultHarnessSignerAddresses returns the Ethereum addresses derived from the
// test private keys.
func DefaultHarnessSignerAddresses() []string {
	signers := DefaultHarnessAttestationSigners()
	addresses := make([]string, 0, len(signers))
	for _, signer := range signers {
		addresses = append(addresses, signer.Address)
	}
	return addresses
}

// DefaultHarnessSignerPrivateKeys returns the raw hex private keys. Never use
// these outside of tests or local devnets.
func DefaultHarnessSignerPrivateKeys() []string {
	keys := make([]string, 0, len(defaultHarnessSignerPrivateKeys))
	keys = append(keys, defaultHarnessSignerPrivateKeys...)
	return keys
}
