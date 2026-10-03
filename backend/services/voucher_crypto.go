package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

func voucherCipher() (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv("VOUCHER_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 {
		return nil, errors.New("voucher storage encryption requires a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func EncryptVoucher(code string) (string, error) {
	g, err := voucherCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	data := g.Seal(nonce, nonce, []byte(code), []byte("prophit-voucher-v1"))
	return "v1:" + base64.StdEncoding.EncodeToString(data), nil
}
func DecryptVoucher(stored string) (string, error) {
	g, err := voucherCipher()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(stored, "v1:") {
		return "", errors.New("historical voucher requires storage reconciliation")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "v1:"))
	if err != nil || len(data) < g.NonceSize() {
		return "", errors.New("invalid voucher ciphertext")
	}
	plain, err := g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], []byte("prophit-voucher-v1"))
	return string(plain), err
}
