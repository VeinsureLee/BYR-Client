package byrhttp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestEncryptedResponseVerificationAndTamperRejection(t *testing.T) {
	client, _ := ecdh.X25519().GenerateKey(rand.Reader)
	server, _ := ecdh.X25519().GenerateKey(rand.Reader)
	public, signing, _ := ed25519.GenerateKey(rand.Reader)
	secret, _ := server.ECDH(client.PublicKey())
	block, _ := aes.NewCipher(secret)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	rand.Read(nonce)
	content := gcm.Seal(nonce, nonce, []byte(`{"ok":true}`), nil)
	e := EncryptedData{Content: base64.StdEncoding.EncodeToString(content), PublicKey: base64.StdEncoding.EncodeToString(server.PublicKey().Bytes()), Sign: base64.StdEncoding.EncodeToString(ed25519.Sign(signing, content))}
	plain, err := decryptExperimental(e, client, public)
	if err != nil || string(plain) != `{"ok":true}` {
		t.Fatalf("%s %v", plain, err)
	}
	content[len(content)-1] ^= 1
	e.Content = base64.StdEncoding.EncodeToString(content)
	if _, err := decryptExperimental(e, client, public); err == nil {
		t.Fatal("accepted modified ciphertext")
	}
	e.Sign = base64.StdEncoding.EncodeToString(ed25519.Sign(signing, content))
	if _, err := decryptExperimental(e, client, public); err == nil {
		t.Fatal("accepted invalid GCM tag despite valid envelope signature")
	}
}
