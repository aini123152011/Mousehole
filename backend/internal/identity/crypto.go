package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
)

type keySet struct {
	loginEncryption []byte
	csrf            []byte
}

func deriveKeys(master []byte) (keySet, error) {
	if len(master) != 32 {
		return keySet{}, errors.New("MOUSEHOLE_AUTH_KEY 解码后必须正好为 32 字节")
	}
	return keySet{
		loginEncryption: derive(master, "mousehole:oidc-login:v1"),
		csrf:            derive(master, "mousehole:csrf:v1"),
	}, nil
}

func derive(master []byte, label string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte(label))
	return mac.Sum(nil)
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func digest(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func encrypt(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(plaintext), []byte("mousehole:pkce:v1")), nil
}

func decrypt(key, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aead.NonceSize() {
		return "", errors.New("加密登录尝试长度无效")
	}
	nonce, payload := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, payload, []byte("mousehole:pkce:v1"))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func csrfToken(key []byte, sessionToken string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func equalToken(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
