package customers

import (
	"fmt"
	"os"
)

// KeyProvider supplies the encryption key for PII data.
type KeyProvider interface {
	PIIKey() ([]byte, error)
}

// FixedKeyProvider returns a hardcoded key. Suitable for WASM/testing.
type FixedKeyProvider struct{ Key []byte }

func (f FixedKeyProvider) PIIKey() ([]byte, error) { return f.Key, nil }

// EnvKeyProvider reads the key from an environment variable.
type EnvKeyProvider struct{ VarName string }

func (e EnvKeyProvider) PIIKey() ([]byte, error) {
	key := os.Getenv(e.VarName)
	if key == "" {
		return nil, fmt.Errorf("env var %s not set", e.VarName)
	}
	return []byte(key), nil
}
