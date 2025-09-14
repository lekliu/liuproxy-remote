// --- cipher.go ---
package securecrypt

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

type Cipher struct {
	aead cipher.AEAD
}

// NewCipher 创建一个硬编码使用 XChaCha20-Poly1305 的加密器。
func NewCipher(key int) (*Cipher, error) {
	keyBytes := []byte(fmt.Sprintf("liuproxy-secure-v2-key-%d", key))
	hash := sha256.Sum256(keyBytes)
	finalKey := hash[:]

	aead, err := newChaCha20AEAD(finalKey)
	if err != nil {
		return nil, err
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt 方法保持不变
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 方法保持不变
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext is too short")
	}
	nonce, encryptedMessage := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, encryptedMessage, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return plaintext, nil
}
