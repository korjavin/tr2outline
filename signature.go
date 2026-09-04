package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// VerifySignature validates that the provided raw payload matches the signature
// header computed using the secret.
// Expected signature header format: "sha256=<hex_hash>" (or plain hex hash).
func VerifySignature(payload []byte, signatureHeader, secret string) bool {
	if signatureHeader == "" || secret == "" {
		return false
	}

	// Strip "sha256=" prefix if present
	actualSignature := strings.TrimSpace(signatureHeader)
	if strings.HasPrefix(strings.ToLower(actualSignature), "sha256=") {
		actualSignature = actualSignature[7:]
	}

	// Compute HMAC-SHA256 of the raw body
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

// Constant-time comparison
	return subtle.ConstantTimeCompare(
		[]byte(strings.ToLower(actualSignature)),
		[]byte(strings.ToLower(expectedSignature)),
	) == 1
}

// VerifyAnySignature validates that the provided raw payload matches the signature
// header computed using any of the provided secrets.
func VerifyAnySignature(payload []byte, signatureHeader string, secrets []string) bool {
	if len(secrets) == 0 || signatureHeader == "" {
		return false
	}
	for _, secret := range secrets {
		if VerifySignature(payload, signatureHeader, secret) {
			return true
		}
	}
	return false
}

// ComputeSignature calculates the HMAC-SHA256 signature formatted as sha256=<hex_hash>.
func ComputeSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
