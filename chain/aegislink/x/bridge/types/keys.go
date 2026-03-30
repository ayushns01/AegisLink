package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
)

func ReplayKey(kind ClaimKind, sourceChainID, sourceContract, sourceTxHash string, sourceLogIndex, nonce uint64) string {
	sum := sha256.Sum256(canonicalIdentityBytes(kind, sourceChainID, sourceContract, sourceTxHash, sourceLogIndex, nonce))
	return hex.EncodeToString(sum[:])
}

func canonicalIdentityBytes(kind ClaimKind, sourceChainID, sourceContract, sourceTxHash string, sourceLogIndex, nonce uint64) []byte {
	var buf bytes.Buffer
	writeCanonicalString(&buf, string(kind))
	writeCanonicalString(&buf, sourceChainID)
	writeCanonicalString(&buf, sourceContract)
	writeCanonicalString(&buf, sourceTxHash)

	var numeric [8]byte
	binary.BigEndian.PutUint64(numeric[:], sourceLogIndex)
	buf.Write(numeric[:])
	binary.BigEndian.PutUint64(numeric[:], nonce)
	buf.Write(numeric[:])

	return buf.Bytes()
}

func writeCanonicalString(buf *bytes.Buffer, value string) {
	normalized := strings.TrimSpace(value)
	var size [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(size[:], uint64(len(normalized)))
	buf.Write(size[:n])
	buf.WriteString(normalized)
}
