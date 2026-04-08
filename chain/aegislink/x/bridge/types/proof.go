package types

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

type AttestationProof struct {
	Signer    string `json:"signer"`
	Signature []byte `json:"signature"`
}

type HarnessSigner struct {
	Address       string `json:"address"`
	PrivateKeyHex string `json:"private_key_hex"`
}

var defaultHarnessSignerPrivateKeys = []string{
	"0000000000000000000000000000000000000000000000000000000000000001",
	"0000000000000000000000000000000000000000000000000000000000000002",
	"0000000000000000000000000000000000000000000000000000000000000003",
	"0000000000000000000000000000000000000000000000000000000000000004",
	"0000000000000000000000000000000000000000000000000000000000000005",
	"0000000000000000000000000000000000000000000000000000000000000006",
}

func DefaultHarnessAttestationSigners() []HarnessSigner {
	signers := make([]HarnessSigner, 0, len(defaultHarnessSignerPrivateKeys))
	for _, key := range defaultHarnessSignerPrivateKeys {
		address, err := SignerAddressFromPrivateKeyHex(key)
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

func DefaultHarnessSignerAddresses() []string {
	signers := DefaultHarnessAttestationSigners()
	addresses := make([]string, 0, len(signers))
	for _, signer := range signers {
		addresses = append(addresses, signer.Address)
	}
	return addresses
}

func DefaultHarnessSignerPrivateKeys() []string {
	keys := make([]string, 0, len(defaultHarnessSignerPrivateKeys))
	keys = append(keys, defaultHarnessSignerPrivateKeys...)
	return keys
}

func (a Attestation) SigningDigest() ([]byte, error) {
	if err := a.validateEnvelope(); err != nil {
		return nil, err
	}

	payload := strings.Join([]string{
		"aegislink.attestation.v1",
		strings.TrimSpace(a.MessageID),
		strings.ToLower(strings.TrimSpace(a.PayloadHash)),
		fmt.Sprintf("%d", a.Threshold),
		fmt.Sprintf("%d", a.Expiry),
		fmt.Sprintf("%d", a.SignerSetVersion),
	}, "|")

	digest := keccak256([]byte(payload))
	return digest, nil
}

func SignAttestationWithPrivateKeyHex(attestation Attestation, privateKeyHex string) (AttestationProof, error) {
	digest, err := attestation.SigningDigest()
	if err != nil {
		return AttestationProof{}, err
	}
	privateKey, err := parsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return AttestationProof{}, err
	}
	return AttestationProof{
		Signer:    SignerAddressFromPublicKey(privateKey.PubKey()),
		Signature: ecdsa.SignCompact(privateKey, digest, false),
	}, nil
}

func SignerAddressFromPrivateKeyHex(privateKeyHex string) (string, error) {
	privateKey, err := parsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return "", err
	}
	return SignerAddressFromPublicKey(privateKey.PubKey()), nil
}

func SignerAddressFromPublicKey(publicKey *secp256k1.PublicKey) string {
	serialized := publicKey.SerializeUncompressed()
	hash := keccak256(serialized[1:])
	return "0x" + hex.EncodeToString(hash[len(hash)-20:])
}

func NormalizeSignerAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func parsePrivateKeyHex(privateKeyHex string) (*secp256k1.PrivateKey, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x")
	if trimmed == "" {
		return nil, fmt.Errorf("%w: missing private key", ErrInvalidAttestation)
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: decode private key: %v", ErrInvalidAttestation, err)
	}
	privateKey := secp256k1.PrivKeyFromBytes(decoded)
	if privateKey.Key.IsZero() {
		return nil, fmt.Errorf("%w: zero private key", ErrInvalidAttestation)
	}
	return privateKey, nil
}

func keccak256(data []byte) []byte {
	digest := sha3.NewLegacyKeccak256()
	_, _ = digest.Write(data)
	return digest.Sum(nil)
}
