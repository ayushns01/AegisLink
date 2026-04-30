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

// SigningDigest returns the canonical digest that attestation signers must sign.
//
// Each field is length-prefixed as "<len>:<value>" before being joined with "|"
// so that no choice of MessageID or PayloadHash can produce the same byte
// sequence as a different (MessageID, PayloadHash) pair — preventing hash
// collision attacks via embedded delimiter characters.
func (a Attestation) SigningDigest() ([]byte, error) {
	if err := a.validateEnvelope(); err != nil {
		return nil, err
	}

	fields := []string{
		"aegislink.attestation.v1",
		strings.TrimSpace(a.MessageID),
		strings.ToLower(strings.TrimSpace(a.PayloadHash)),
		fmt.Sprintf("%d", a.Threshold),
		fmt.Sprintf("%d", a.Expiry),
		fmt.Sprintf("%d", a.SignerSetVersion),
	}

	// Length-prefix every field: "<len>:<value>". This ensures the serialization
	// is unambiguous even when fields contain the "|" separator character.
	encoded := make([]string, len(fields))
	for i, f := range fields {
		encoded[i] = fmt.Sprintf("%d:%s", len(f), f)
	}
	payload := strings.Join(encoded, "|")

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

// SignRawPayload signs an arbitrary byte payload with the given secp256k1
// private key and returns a compact signature. Use for non-attestation domains
// (e.g. vote signing) where a custom domain-separation prefix is baked into
// the payload by the caller.
func SignRawPayload(payload []byte, privateKeyHex string) ([]byte, error) {
	privateKey, err := parsePrivateKeyHex(privateKeyHex)
	if err != nil {
		return nil, err
	}
	digest := keccak256(payload)
	return ecdsa.SignCompact(privateKey, digest, false), nil
}

// RecoverSignerFromPayload recovers the Ethereum address of the signer of
// payload given a compact secp256k1 signature produced by SignRawPayload.
func RecoverSignerFromPayload(payload []byte, signature []byte) (string, error) {
	if len(signature) == 0 {
		return "", fmt.Errorf("%w: empty signature", ErrInvalidAttestation)
	}
	digest := keccak256(payload)
	publicKey, _, err := ecdsa.RecoverCompact(signature, digest)
	if err != nil {
		return "", fmt.Errorf("%w: recover signer: %v", ErrInvalidAttestation, err)
	}
	return SignerAddressFromPublicKey(publicKey), nil
}

func keccak256(data []byte) []byte {
	digest := sha3.NewLegacyKeccak256()
	_, _ = digest.Write(data)
	return digest.Sum(nil)
}
