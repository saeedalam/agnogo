package tools

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/saeedalam/agnogo"
)

// Crypto returns tools for AES-256-GCM encryption and decryption.
func Crypto() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "generate_key", Desc: "Generate a random 256-bit hex key for AES encryption",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				key := make([]byte, 32)
				if _, err := rand.Read(key); err != nil {
					return "", fmt.Errorf("failed to generate key: %w", err)
				}
				return fmt.Sprintf(`{"key":"%s"}`, hex.EncodeToString(key)), nil
			},
		},
		{
			Name: "encrypt", Desc: "Encrypt data with AES-256-GCM",
			Params: agnogo.Params{
				"key":       {Type: "string", Desc: "32-byte hex-encoded key (64 hex chars)", Required: true},
				"plaintext": {Type: "string", Desc: "Data to encrypt", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				keyHex := args["key"]
				plaintext := args["plaintext"]
				if keyHex == "" || plaintext == "" {
					return "", fmt.Errorf("key and plaintext are required")
				}
				keyBytes, err := hex.DecodeString(keyHex)
				if err != nil {
					return "", fmt.Errorf("invalid hex key: %w", err)
				}
				if len(keyBytes) != 32 {
					return "", fmt.Errorf("key must be 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
				}

				block, err := aes.NewCipher(keyBytes)
				if err != nil {
					return "", fmt.Errorf("cipher init: %w", err)
				}
				gcm, err := cipher.NewGCM(block)
				if err != nil {
					return "", fmt.Errorf("GCM init: %w", err)
				}

				nonce := make([]byte, gcm.NonceSize())
				if _, err := rand.Read(nonce); err != nil {
					return "", fmt.Errorf("nonce generation: %w", err)
				}

				ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
				result := map[string]string{"ciphertext": hex.EncodeToString(ciphertext)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "decrypt", Desc: "Decrypt AES-256-GCM encrypted data",
			Params: agnogo.Params{
				"key":        {Type: "string", Desc: "32-byte hex-encoded key (64 hex chars)", Required: true},
				"ciphertext": {Type: "string", Desc: "Hex-encoded nonce+ciphertext from encrypt", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				keyHex := args["key"]
				ctHex := args["ciphertext"]
				if keyHex == "" || ctHex == "" {
					return "", fmt.Errorf("key and ciphertext are required")
				}
				keyBytes, err := hex.DecodeString(keyHex)
				if err != nil {
					return "", fmt.Errorf("invalid hex key: %w", err)
				}
				if len(keyBytes) != 32 {
					return "", fmt.Errorf("key must be 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
				}
				ctBytes, err := hex.DecodeString(ctHex)
				if err != nil {
					return "", fmt.Errorf("invalid hex ciphertext: %w", err)
				}

				block, err := aes.NewCipher(keyBytes)
				if err != nil {
					return "", fmt.Errorf("cipher init: %w", err)
				}
				gcm, err := cipher.NewGCM(block)
				if err != nil {
					return "", fmt.Errorf("GCM init: %w", err)
				}

				nonceSize := gcm.NonceSize()
				if len(ctBytes) < nonceSize {
					return "", fmt.Errorf("ciphertext too short")
				}
				nonce, ct := ctBytes[:nonceSize], ctBytes[nonceSize:]
				plaintext, err := gcm.Open(nil, nonce, ct, nil)
				if err != nil {
					return "", fmt.Errorf("decryption failed: %w", err)
				}
				result := map[string]string{"plaintext": string(plaintext)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
