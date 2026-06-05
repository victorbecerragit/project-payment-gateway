package id

import (
	"time"
)

// GeneratePaymentID generates a unique identifier for a payment
func GeneratePaymentID() string {
	return "pay_" + time.Now().Format("20060102150405")
}

// GenerateTransactionID generates a unique identifier for a transaction
func GenerateTransactionID() string {
	return "txn_" + time.Now().Format("20060102150405")
}
