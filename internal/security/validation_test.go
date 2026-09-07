package security

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestSanitizePlayerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "trims whitespace", input: "  Player  ", want: "Player"},
		{name: "accepts unicode", input: "玩家一号", want: "玩家一号"},
		{name: "rejects short name", input: "A", wantErr: true},
		{name: "rejects invisible rune", input: "Pl\u200Bayer", wantErr: true},
		{name: "rejects control rune", input: "Play\ner", wantErr: true},
		{name: "rejects trailing control rune", input: "Player\n", wantErr: true},
		{name: "rejects invalid UTF-8", input: string([]byte{'P', 0xff, 'r', 'o'}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePlayerName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SanitizePlayerName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("SanitizePlayerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateScore(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	valid := []struct {
		name       string
		score      int64
		durationMS int
		timestamp  int64
	}{
		{name: "normal score", score: 600, durationMS: 5000, timestamp: now.Unix()},
		{name: "boundary score", score: 600, durationMS: 3000, timestamp: now.Unix()},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateScore(tt.score, tt.durationMS, tt.timestamp, now); err != nil {
				t.Fatalf("ValidateScore() error = %v", err)
			}
		})
	}

	if err := ValidateScore(601, 3000, now.Unix(), now); err == nil {
		t.Fatal("ValidateScore() accepted an impossible score")
	}
	if err := ValidateScore(1, 3000, now.Unix()+121, now); err == nil {
		t.Fatal("ValidateScore() accepted excessive timestamp drift")
	}
}

func TestSessionSecretDecryptsAES256Payload(t *testing.T) {
	secret, err := NewSessionSecret()
	if err != nil {
		t.Fatalf("NewSessionSecret() error = %v", err)
	}
	encoded, err := mustEncryptSessionPayload(secret, []byte("score payload"))
	if err != nil {
		t.Fatalf("encrypt payload error = %v", err)
	}
	plain, err := DecryptAESGCM(secret, encoded)
	if err != nil {
		t.Fatalf("DecryptAESGCM() error = %v", err)
	}
	if string(plain) != "score payload" {
		t.Fatalf("DecryptAESGCM() = %q", plain)
	}
}

func mustEncryptSessionPayload(secret string, plaintext []byte) (string, error) {
	key, err := hex.DecodeString(secret)
	if err != nil {
		return "", err
	}
	return encrypt(key, plaintext)
}
