package security

import "testing"

func TestSecretEnvelopeRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 7
	encoded, err := EncryptSecret(key, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("EncryptSecret() error = %v", err)
	}
	decoded, err := DecryptSecret(key, encoded)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if decoded != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("DecryptSecret() = %q", decoded)
	}
	if _, err := DecryptSecret(make([]byte, 32), encoded); err == nil {
		t.Fatal("DecryptSecret() accepted the wrong key")
	}
	if _, err := DecryptSecret(make([]byte, 16), encoded); err == nil {
		t.Fatal("DecryptSecret() accepted a non-AES-256 key")
	}
}
