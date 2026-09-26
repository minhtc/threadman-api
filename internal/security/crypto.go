package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

const AES256KeySize = 32

func NewSessionSecret() (string, error) {
	secret := make([]byte, AES256KeySize)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

func EncryptSecret(masterKey []byte, plaintext string) (string, error) {
	return encrypt(masterKey, []byte(plaintext))
}

func DecryptSecret(masterKey []byte, encoded string) (string, error) {
	plain, err := decrypt(masterKey, encoded)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// EncryptAESGCM encrypts plaintext with the hex-encoded session secret. It is
// the inverse of DecryptAESGCM and mirrors what game clients do, so tests can
// build a payload the service will accept.
func EncryptAESGCM(secretHex string, plaintext []byte) (string, error) {
	key, err := hex.DecodeString(secretHex)
	if err != nil || len(key) != AES256KeySize {
		return "", errors.New("invalid AES-256 session key")
	}
	return encrypt(key, plaintext)
}

func DecryptAESGCM(secretHex, encoded string) ([]byte, error) {
	key, err := hex.DecodeString(secretHex)
	if err != nil || len(key) != AES256KeySize {
		return nil, errors.New("invalid AES-256 session key")
	}
	return decrypt(key, encoded)
}

func encrypt(key []byte, plaintext []byte) (string, error) {
	if len(key) != AES256KeySize {
		return "", errors.New("encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	data := append(nonce, gcm.Seal(nil, nonce, plaintext, nil)...)
	return base64.StdEncoding.EncodeToString(data), nil
}

func decrypt(key []byte, encoded string) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("base64 decoding failed")
	}
	if len(data) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("payload data truncated")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}
