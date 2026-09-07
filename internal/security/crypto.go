package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

func NewSessionSecret() (string, error) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

func DecryptAESGCM(secretHex, encoded string) ([]byte, error) {
	key, err := hex.DecodeString(secretHex)
	if err != nil || len(key) != 16 {
		return nil, errors.New("invalid session key")
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("base64 decoding failed")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("payload data truncated")
	}

	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}
