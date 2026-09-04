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
