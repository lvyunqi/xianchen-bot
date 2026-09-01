package util

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func SecureEqualHash(token, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(TokenHash(token)), []byte(hash)) == 1
}
