package tools

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/saeedalam/agnogo"
)

// Hash returns a tool for computing cryptographic hashes and HMACs.
func Hash() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "hash", Desc: "Compute cryptographic hash or HMAC of data",
			Params: agnogo.Params{
				"algorithm": {Type: "string", Desc: "Hash algorithm", Required: true, Enum: []string{"sha256", "sha512", "md5", "sha1"}},
				"data":      {Type: "string", Desc: "Data to hash", Required: true},
				"key":       {Type: "string", Desc: "Optional hex-encoded key for HMAC"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				algorithm := args["algorithm"]
				data := args["data"]
				key := args["key"]
				if data == "" {
					return "", fmt.Errorf("data is required")
				}

				newHash := func() hash.Hash {
					switch algorithm {
					case "sha256":
						return sha256.New()
					case "sha512":
						return sha512.New()
					case "md5":
						return md5.New()
					case "sha1":
						return sha1.New()
					default:
						return nil
					}
				}

				if newHash() == nil {
					return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
				}

				var h hash.Hash
				if key != "" {
					keyBytes, err := hex.DecodeString(key)
					if err != nil {
						return "", fmt.Errorf("invalid hex key: %w", err)
					}
					h = hmac.New(newHash, keyBytes)
				} else {
					h = newHash()
				}

				h.Write([]byte(data))
				result := hex.EncodeToString(h.Sum(nil))
				mode := "hash"
				if key != "" {
					mode = "hmac"
				}
				return fmt.Sprintf(`{"algorithm":"%s","mode":"%s","hash":"%s"}`, algorithm, mode, result), nil
			},
		},
	}
}
