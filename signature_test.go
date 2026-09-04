package main

import "testing"

func TestSignatureVerification(t *testing.T) {
	secret := "super-secret-key-123"
	payload := []byte(`{"event":"note.enhanced","data":{"meeting":{"title":"Test"}}}`)

	validSig := ComputeSignature(payload, secret)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		expected  bool
	}{
		{
			name:      "Valid signature with sha256= prefix",
			payload:   payload,
			signature: validSig,
			secret:    secret,
			expected:  true,
		},
		{
			name:      "Valid signature without sha256= prefix",
			payload:   payload,
			signature: validSig[7:], // remove "sha256="
			secret:    secret,
			expected:  true,
		},
		{
			name:      "Uppercase sha256= prefix and hex",
			payload:   payload,
			signature: "SHA256=" + validSig[7:],
			secret:    secret,
			expected:  true,
		},
		{
			name:      "Tampered payload",
			payload:   []byte(`{"event":"tampered"}`),
			signature: validSig,
			secret:    secret,
			expected:  false,
		},
		{
			name:      "Wrong secret",
			payload:   payload,
			signature: validSig,
			secret:    "different-secret",
			expected:  false,
		},
		{
			name:      "Empty signature header",
			payload:   payload,
			signature: "",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "Empty secret",
			payload:   payload,
			signature: validSig,
			secret:    "",
			expected:  false,
		},
		{
			name:      "Malformed signature string",
			payload:   payload,
			signature: "sha256=not-a-valid-hex",
			secret:    secret,
			expected:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := VerifySignature(tc.payload, tc.signature, tc.secret)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestVerifyAnySignature(t *testing.T) {
	payload := []byte(`{"test":"multiple_secrets"}`)
	secret1 := "device-phone-secret"
	secret2 := "device-laptop-secret"
	secret3 := "device-tablet-secret"
	secrets := []string{secret1, secret2, secret3}

	sig1 := ComputeSignature(payload, secret1)
	sig2 := ComputeSignature(payload, secret2)
	sig3 := ComputeSignature(payload, secret3)
	sigUnknown := ComputeSignature(payload, "unregistered-secret")

	if !VerifyAnySignature(payload, sig1, secrets) {
		t.Errorf("expected signature from secret1 to match")
	}
	if !VerifyAnySignature(payload, sig2, secrets) {
		t.Errorf("expected signature from secret2 to match")
	}
	if !VerifyAnySignature(payload, sig3, secrets) {
		t.Errorf("expected signature from secret3 to match")
	}
	if VerifyAnySignature(payload, sigUnknown, secrets) {
		t.Errorf("expected signature from unknown secret to fail")
	}
	if VerifyAnySignature(payload, sig1, nil) {
		t.Errorf("expected nil secrets to fail")
	}
	if VerifyAnySignature(payload, "", secrets) {
		t.Errorf("expected empty signature to fail")
	}
}

