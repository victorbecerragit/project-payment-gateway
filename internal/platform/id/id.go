package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback keeps IDs usable even if CSPRNG is unavailable.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GeneratePaymentID generates a unique identifier for a payment
func GeneratePaymentID() string {
	return fmt.Sprintf("pay_%d_%s", time.Now().UnixNano(), randomHex(4))
}

// GenerateTransactionID generates a unique identifier for a transaction
func GenerateTransactionID() string {
	return fmt.Sprintf("txn_%d_%s", time.Now().UnixNano(), randomHex(4))
}
