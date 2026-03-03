package workspace

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// AESGCMEncryptor encrypts sensitive workspace key material for at-rest storage.
type AESGCMEncryptor struct {
	aead cipher.AEAD
}

func NewAESGCMEncryptor(rawKey []byte) (*AESGCMEncryptor, error) {
	if l := len(rawKey); l != 16 && l != 24 && l != 32 {
		return nil, fmt.Errorf("invalid AES key length: %d", l)
	}

	block, err := aes.NewCipher(rawKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCMEncryptor{aead: aead}, nil
}

func NewAESGCMEncryptorFromBase64(keyB64 string) (*AESGCMEncryptor, error) {
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("decode base64 encryption key: %w", err)
	}
	return NewAESGCMEncryptor(raw)
}

func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := e.aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func (e *AESGCMEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	enc := ciphertext[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, enc, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
