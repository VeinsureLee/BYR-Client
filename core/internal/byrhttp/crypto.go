package byrhttp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

const experimentalVerifyKey = "qtjZC1hWINhsGfWlPD6vK8WoaGtWUY4XRpusjw+2kOE="

type EncryptedData struct {
	Content   string `json:"content"`
	PublicKey string `json:"public_key"`
	Sign      string `json:"sign"`
}

func (c *Client) ExperimentalStatus(ctx context.Context) (json.RawMessage, error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	var envelope EncryptedData
	err = c.request(ctx, http.MethodGet, "exp", []string{"status"}, url.Values{"key": {base64.StdEncoding.EncodeToString(key.PublicKey().Bytes())}}, nil, "", true, &envelope)
	if err != nil {
		return nil, err
	}
	verify, err := base64.StdEncoding.DecodeString(experimentalVerifyKey)
	if err != nil {
		return nil, errors.New("invalid experimental verification key")
	}
	return decryptExperimental(envelope, key, verify)
}
func decryptExperimental(e EncryptedData, key *ecdh.PrivateKey, verify ed25519.PublicKey) (json.RawMessage, error) {
	content, err := base64.StdEncoding.DecodeString(e.Content)
	if err != nil {
		return nil, errors.New("invalid encrypted content")
	}
	sign, err := base64.StdEncoding.DecodeString(e.Sign)
	if err != nil || len(verify) != ed25519.PublicKeySize || !ed25519.Verify(verify, content, sign) {
		return nil, errors.New("加密响应签名验证失败")
	}
	remote, err := base64.StdEncoding.DecodeString(e.PublicKey)
	if err != nil {
		return nil, errors.New("invalid server public key")
	}
	public, err := ecdh.X25519().NewPublicKey(remote)
	if err != nil {
		return nil, err
	}
	secret, err := key.ECDH(public)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(content) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("encrypted response is truncated")
	}
	plain, err := gcm.Open(nil, content[:gcm.NonceSize()], content[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("加密响应解密失败")
	}
	if !json.Valid(plain) {
		plain, err = json.Marshal(string(plain))
	}
	return plain, err
}
